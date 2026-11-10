package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"voiceotp/gateway/ami"
	"voiceotp/gateway/internal/logging"
)

// Server is the VoiceOTP HTTP API server.
type Server struct {
	cfg         *Config
	logger      *logging.Logger
	creds       *Credentials
	amiCfg      AMIConfig
	astCfg      AsteriskConfig
	secCfg      SecurityConfig
	rateLimiter *RateLimiter
	idempotent  *IdempotencyStore
	callSlot    *CallSlot
	httpServer  *http.Server
	mu          sync.Mutex
}

// NewServer builds a Server from a validated config.
func NewServer(cfg *Config, logger *logging.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Server{
		cfg:         cfg,
		logger:      logger,
		creds:       NewCredentials(cfg.API.Username, cfg.API.Password),
		amiCfg:      cfg.AMI,
		astCfg:      cfg.Asterisk,
		secCfg:      cfg.Security,
		rateLimiter: NewRateLimiter(cfg.Security.MaxRequestsPerMinute, time.Minute),
		idempotent:  NewIdempotencyStore(5 * time.Minute),
		callSlot:    NewCallSlot(cfg.Security.MaxConcurrentCalls),
	}
	go s.rateLimiter.pruneLoop()
	go s.idempotent.pruneLoop()
	return s, nil
}

func (s *Server) pruneLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.rateLimiter.prune()
		s.idempotent.prune()
	}
}

// Handler returns the http.Handler for the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/voice-otp", s.handleVoiceOTP)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return mux
}

// ListenAndServe starts the HTTP (or HTTPS) server and blocks.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Server.Listen
	var err error
	if s.cfg.Server.TLSEnabled {
		s.httpServer = &http.Server{
			Addr:              addr,
			Handler:           s.Handler(),
			ReadHeaderTimeout: time.Duration(s.cfg.Server.ReadTimeoutSeconds) * time.Second,
		}
		s.httpServer.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		err = s.httpServer.ListenAndServeTLS(s.cfg.Server.TLSCert, s.cfg.Server.TLSKey)
	} else {
		s.httpServer = &http.Server{
			Addr:              addr,
			Handler:           s.Handler(),
			ReadHeaderTimeout: time.Duration(s.cfg.Server.ReadTimeoutSeconds) * time.Second,
		}
		err = s.httpServer.ListenAndServe()
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops the HTTP server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleVoiceOTP(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID(r)
	w.Header().Set("Content-Type", "application/json")

	// IP allowlist.
	ip := clientIP(r.RemoteAddr)
	if !IPAllowed(ip, s.secCfg.AllowedClientCIDRs) {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "ip": ip, "event": "ip_blocked"})
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "forbidden"})
		return
	}

	// Rate limiting.
	if !s.rateLimiter.Allow(requestID) {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "ip": ip, "event": "rate_limited"})
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "rate limit exceeded"})
		return
	}

	// Idempotency (optional Idempotency-Key header).
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		if s.idempotent.Seen(key) {
			s.logger.Info(map[string]interface{}{"request_id": requestID, "event": "idempotent_duplicate"})
			writeJSON(w, http.StatusConflict, map[string]interface{}{"ok": false, "error": "duplicate request"})
			return
		}
	}

	// Read and decode body.
	body, err := readBody(r, int64(s.cfg.API.MaxRequestBodyBytes))
	if err != nil {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "event": "bad_request", "reason": err.Error()})
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid request body"})
		return
	}
	var req RequestBody
	if err := json.Unmarshal(body, &req); err != nil {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "event": "bad_request", "reason": "malformed json"})
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid json"})
		return
	}

	// Authenticate (constant-time).
	if !s.creds.Auth(req.Username, req.Password) {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "ip": ip, "event": "auth_failed"})
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"ok": false, "error": "authentication failed"})
		return
	}

	// Validate fields.
	if err := ValidateRequest(req, &s.cfg.API); err != nil {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "event": "validation_failed", "reason": err.Error()})
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	number, err := NormalizeNumber(req.Number)
	if err != nil {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "event": "validation_failed", "reason": err.Error()})
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid number"})
		return
	}

	// Acquire concurrent-call slot.
	if !s.callSlot.Acquire() {
		s.logger.Warn(map[string]interface{}{"request_id": requestID, "event": "call_slot_full"})
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"ok": false, "error": "too many concurrent calls"})
		return
	}
	defer s.callSlot.Release()

	// Submit originate to AMI.
	dialString := s.cfg.DialString(number)
	variables := map[string]string{
		"VOICEOTP_CODE":       req.OTP,
		"VOICEOTP_LANGUAGE":   s.astCfg.Language,
	}
	amiResp, err := s.originate(requestID, dialString, variables)
	if err != nil {
		s.logger.Error(map[string]interface{}{"request_id": requestID, "number": MaskNumber(number), "event": "ami_error", "reason": err.Error()})
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"ok": false, "error": "ami unavailable"})
		return
	}
	if !ami.IsSuccess(amiResp) {
		s.logger.Error(map[string]interface{}{"request_id": requestID, "number": MaskNumber(number), "event": "originate_rejected", "response": amiResp.Headers["Response"], "message": amiResp.Headers["Message"]})
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"ok": false, "error": "call rejected"})
		return
	}

	s.logger.Info(map[string]interface{}{"request_id": requestID, "number": MaskNumber(number), "event": "originate_queued"})
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"ok":      true,
		"status":  "queued",
		"request_id": requestID,
	})
}

func (s *Server) newRequestID(r *http.Request) string {
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		return "req-" + key
	}
	return GenerateRequestID()
}

func (s *Server) originate(actionID, channel string, variables map[string]string) (*ami.Frame, error) {
	timeout := time.Duration(s.amiCfg.TimeoutSeconds) * time.Second
	c, err := ami.Dial(ami.Config{
		Host:     s.amiCfg.Host,
		Port:     s.amiCfg.Port,
		Username: s.amiCfg.Username,
		Secret:   s.amiCfg.Secret,
		Timeout:  timeout,
	})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.SendOriginate(actionID, channel, s.astCfg.Context, "s", s.astCfg.CallerID, s.astCfg.CallTimeoutMS, variables)
}

// clientIP extracts the IP from RemoteAddr.
func clientIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return net.ParseIP(remoteAddr)
	}
	return net.ParseIP(host)
}

func readBody(r *http.Request, maxBytes int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	return ioReadAll(r.Body)
}

func ioReadAll(r io.Reader) ([]byte, error) {
	// Avoid importing io package twice; use a small buffer.
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 512)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

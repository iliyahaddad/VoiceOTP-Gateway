package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"voiceotp/gateway/internal/logging"
)

// fakeAMIServer is a minimal TCP AMI server for handler tests.
type fakeAMIServer struct {
	listener net.Listener
	port     int
	mu       sync.Mutex
	originate map[string]string // actionID -> raw frame text
	t        *testing.T
}

func startFakeAMIServer(t *testing.T) *fakeAMIServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeAMIServer{listener: ln, port: ln.Addr().(*net.TCPAddr).Port, originate: make(map[string]string), t: t}
	go f.serve()
	t.Cleanup(f.close)
	return f
}

func (f *fakeAMIServer) close() { _ = f.listener.Close() }

func (f *fakeAMIServer) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeAMIServer) handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writer.WriteString("Asterisk Call Manager/20.0.0\r\n\r\n")
	writer.Flush()
	for {
		frame, err := readAMIFrame(reader)
		if err != nil {
			return
		}
		action := frame.Headers["Action"]
		if action == "Login" {
			writer.WriteString("Response: Success\r\nMessage: Authentication accepted\r\n\r\n")
			writer.Flush()
			continue
		}
		if action == "Originate" {
			actionID := frame.Headers["ActionID"]
			f.mu.Lock()
			f.originate[actionID] = frame.Raw
			f.mu.Unlock()
			writer.WriteString("Response: Success\r\nMessage: Originate queued\r\n\r\n")
			writer.Flush()
			continue
		}
		if action == "Logoff" {
			return
		}
	}
}

func (f *fakeAMIServer) originateHeaders(actionID string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.originate[actionID]
	return h, ok
}

func readAMIFrame(reader *bufio.Reader) (struct{ Headers map[string]string; Raw string }, error) {
	frame := struct{ Headers map[string]string; Raw string }{Headers: make(map[string]string)}
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return frame, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
		idx := strings.Index(line, ":")
		if idx >= 0 {
			frame.Headers[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
		}
	}
	frame.Raw = strings.Join(lines, "\r\n")
	return frame, nil
}

func makeGatewayServer(t *testing.T, f *fakeAMIServer) *httptest.Server {
	t.Helper()
	cfg := &Config{
		Server: ServerConfig{Listen: "127.0.0.1:0", TLSEnabled: false},
		API: APIConfig{Username: "voiceotp_api", Password: "secret", OTPMinLength: 4, OTPMaxLength: 8, MaxRequestBodyBytes: 4096},
		AMI: AMIConfig{Host: "127.0.0.1", Port: f.port, Username: "ami_user", Secret: "ami_secret", TimeoutSeconds: 2},
		Asterisk: AsteriskConfig{ChannelDriver: "pjsip", TrunkName: "trunk1", DialFormat: "PJSIP/trunk1/%s", Context: "voiceotp-playback", CallerID: "VoiceOTP", Language: "en", CallTimeoutMS: 30000},
		Security: SecurityConfig{AllowedClientCIDRs: []string{"127.0.0.1/32"}, MaxRequestsPerMinute: 100, MaxConcurrentCalls: 1000},
	}
	srv, err := NewServer(cfg, logging.Stdout(logging.LevelWarn))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return httptest.NewServer(srv.Handler())
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

func postJSON(t *testing.T, srv *httptest.Server, body string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/voice-otp", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	return httpClient.Do(req)
}

func TestValidRequestAccepted(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"secret","otp":"004829","number":"+989121234567"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d want 202", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("ok not true: %v", out)
	}
	if out["status"] != "queued" {
		t.Fatalf("status not queued: %v", out)
	}
	if _, ok := out["request_id"]; !ok {
		t.Fatal("missing request_id")
	}
	time.Sleep(300 * time.Millisecond)
}

func TestWrongPassword(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"wrong","otp":"004829","number":"+989121234567"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", resp.StatusCode)
	}
}

func TestMissingOTP(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"secret","number":"+989121234567"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}

func TestAlphabeticOTPRejected(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"secret","otp":"ABCD","number":"+989121234567"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}

func TestInvalidNumberRejected(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"secret","otp":"004829","number":"sip:evil@x"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}

func TestAMIDownReturns502(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Listen: "127.0.0.1:0", TLSEnabled: false},
		API: APIConfig{Username: "u", Password: "p", OTPMinLength: 4, OTPMaxLength: 8, MaxRequestBodyBytes: 4096},
		AMI: AMIConfig{Host: "127.0.0.1", Port: 1, Username: "u", Secret: "s", TimeoutSeconds: 1},
		Asterisk: AsteriskConfig{ChannelDriver: "pjsip", TrunkName: "t", DialFormat: "PJSIP/t/%s", CallerID: "v"},
		Security: SecurityConfig{AllowedClientCIDRs: []string{"127.0.0.1/32"}, MaxRequestsPerMinute: 100, MaxConcurrentCalls: 1000},
	}
	gw, err := NewServer(cfg, logging.Stdout(logging.LevelWarn))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	testSrv := httptest.NewServer(gw.Handler())
	defer testSrv.Close()
	body := `{"username":"u","password":"p","otp":"004829","number":"+989121234567"}`
	resp, err := postJSON(t, testSrv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d want 502", resp.StatusCode)
	}
}

func TestLeadingZerosPreservedInAMIVariable(t *testing.T) {
	f := startFakeAMIServer(t)
	srv := makeGatewayServer(t, f)
	defer srv.Close()
	body := `{"username":"voiceotp_api","password":"secret","otp":"004829","number":"+989121234567"}`
	resp, err := postJSON(t, srv, body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d want 202", resp.StatusCode)
	}
	time.Sleep(500 * time.Millisecond)
	f.mu.Lock()
	raws := f.originate
	f.mu.Unlock()
	found := false
	for _, raw := range raws {
		if strings.Contains(raw, "Variable: VOICEOTP_CODE: 004829") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("VOICEOTP_CODE=004829 not found in AMI originate; raws=%v", raws)
	}
}

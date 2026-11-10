package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Config is the top-level VoiceOTP gateway configuration.
type Config struct {
	Server    ServerConfig    `json:"server"`
	API       APIConfig       `json:"api"`
	AMI       AMIConfig       `json:"ami"`
	Asterisk  AsteriskConfig  `json:"asterisk"`
	Security  SecurityConfig  `json:"security"`
}

// ServerConfig holds HTTP/TLS server settings.
type ServerConfig struct {
	Listen                 string `json:"listen"`
	TLSEnabled             bool   `json:"tls_enabled"`
	TLSCert                string `json:"tls_cert"`
	TLSKey                 string `json:"tls_key"`
	ReadTimeoutSeconds     int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds    int    `json:"write_timeout_seconds"`
}

// APIConfig holds API authentication and OTP validation settings.
type APIConfig struct {
	Username             string `json:"username"`
	Password             string `json:"password"`
	OTPMinLength         int    `json:"otp_min_length"`
	OTPMaxLength         int    `json:"otp_max_length"`
	MaxRequestBodyBytes  int    `json:"max_request_body_bytes"`
}

// AMIConfig holds Asterisk Manager Interface settings.
type AMIConfig struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Secret        string `json:"secret"`
	TimeoutSeconds int   `json:"timeout_seconds"`
}

// AsteriskConfig holds dialplan and trunk settings.
type AsteriskConfig struct {
	ChannelDriver string `json:"channel_driver"`
	TrunkName     string `json:"trunk_name"`
	DialFormat    string `json:"dial_format"`
	Context       string `json:"context"`
	CallerID      string `json:"caller_id"`
	Language      string `json:"language"`
	CallTimeoutMS int    `json:"call_timeout_ms"`
}

// SecurityConfig holds IP allowlist and rate-limit settings.
type SecurityConfig struct {
	AllowedClientCIDRs     []string `json:"allowed_client_cidrs"`
	MaxRequestsPerMinute   int      `json:"max_requests_per_minute"`
	MaxConcurrentCalls     int      `json:"max_concurrent_calls"`
}

// LoadConfig reads and parses the JSON configuration file at path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

// Validate checks the configuration for completeness and sanity.
func (c *Config) Validate() error {
	if err := c.Server.validate(); err != nil {
		return err
	}
	if err := c.API.validate(); err != nil {
		return err
	}
	if err := c.AMI.validate(); err != nil {
		return err
	}
	if err := c.Asterisk.validate(); err != nil {
		return err
	}
	if err := c.Security.validate(); err != nil {
		return err
	}
	if c.API.Password == c.AMI.Secret {
		return fmt.Errorf("api.password and ami.secret must be independent credentials")
	}
	return nil
}

func (s ServerConfig) validate() error {
	if s.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if s.TLSEnabled {
		if s.TLSCert == "" {
			return fmt.Errorf("server.tls_cert is required when tls_enabled is true")
		}
		if s.TLSKey == "" {
			return fmt.Errorf("server.tls_key is required when tls_enabled is true")
		}
	}
	if s.ReadTimeoutSeconds <= 0 {
		s.ReadTimeoutSeconds = 5
	}
	if s.WriteTimeoutSeconds <= 0 {
		s.WriteTimeoutSeconds = 10
	}
	return nil
}

func (a APIConfig) validate() error {
	if a.Username == "" {
		return fmt.Errorf("api.username is required")
	}
	if a.Password == "" || a.Password == "CHANGE_THIS" {
		return fmt.Errorf("api.password must be set to a high-entropy secret")
	}
	if a.OTPMinLength <= 0 {
		a.OTPMinLength = 4
	}
	if a.OTPMaxLength < a.OTPMinLength {
		return fmt.Errorf("api.otp_max_length must be >= api.otp_min_length")
	}
	if a.MaxRequestBodyBytes <= 0 {
		a.MaxRequestBodyBytes = 4096
	}
	return nil
}

func (a AMIConfig) validate() error {
	if a.Host == "" {
		return fmt.Errorf("ami.host is required")
	}
	if a.Port <= 0 || a.Port > 65535 {
		return fmt.Errorf("ami.port must be between 1 and 65535")
	}
	if a.Username == "" {
		return fmt.Errorf("ami.username is required")
	}
	if a.Secret == "" || a.Secret == "CHANGE_THIS" {
		return fmt.Errorf("ami.secret must be set to a high-entropy secret")
	}
	if a.TimeoutSeconds <= 0 {
		a.TimeoutSeconds = 3
	}
	return nil
}

func (a AsteriskConfig) validate() error {
	switch a.ChannelDriver {
	case "pjsip", "sip":
	default:
		return fmt.Errorf("asterisk.channel_driver must be 'pjsip' or 'sip'")
	}
	if a.TrunkName == "" || a.TrunkName == "YOUR_TRUNK" {
		return fmt.Errorf("asterisk.trunk_name must be set to your outbound trunk")
	}
	if a.DialFormat == "" || a.DialFormat == "CONFIGURE_FOR_YOUR_TRUNK" {
		return fmt.Errorf("asterisk.dial_format must be set for your trunk")
	}
	if a.Context == "" {
		a.Context = "voiceotp-playback"
	}
	if a.CallerID == "" || a.CallerID == "CONFIGURED_CALLER_ID" {
		return fmt.Errorf("asterisk.caller_id must be set")
	}
	if a.Language == "" {
		a.Language = "en"
	}
	if a.CallTimeoutMS <= 0 {
		a.CallTimeoutMS = 30000
	}
	return nil
}

func (s SecurityConfig) validate() error {
	if len(s.AllowedClientCIDRs) == 0 {
		return fmt.Errorf("security.allowed_client_cidrs must not be empty")
	}
	for _, cidr := range s.AllowedClientCIDRs {
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("security.allowed_client_cidrs contains invalid CIDR %q: %w", cidr, err)
		}
	}
	if s.MaxRequestsPerMinute <= 0 {
		s.MaxRequestsPerMinute = 10
	}
	if s.MaxConcurrentCalls <= 0 {
		s.MaxConcurrentCalls = 5
	}
	return nil
}

// DialString builds the AMI Channel value from configuration using the
// configured dial_format. It never uses caller-supplied input.
func (c *Config) DialString(number string) string {
	format := c.Asterisk.DialFormat
	if format == "" {
		format = c.Asterisk.ChannelDriver + "/" + c.Asterisk.TrunkName + "/%s"
	}
	return strings.ReplaceAll(format, "%s", number)
}

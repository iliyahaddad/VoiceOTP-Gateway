// Package ami implements a minimal Asterisk Manager Interface client.
package ami

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Frame is a parsed AMI frame (response, event or greeting).
type Frame struct {
	Headers map[string]string
	Raw     string
}

// Config holds AMI connection parameters.
type Config struct {
	Host      string
	Port      int
	Username  string
	Secret    string
	Timeout   time.Duration
	TLS       bool
}

// ReadWriter is an AMI connection with a buffered reader/writer.
type ReadWriter struct {
	conn net.Conn
	rw   *bufio.ReadWriter
	mu   sync.Mutex
	closed bool
}

// Dial connects to AMI, reads the greeting, authenticates and returns a ReadWriter.
func Dial(cfg Config) (*ReadWriter, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var d net.Dialer
	var conn net.Conn
	var err error
	if cfg.TLS {
		conn, err = tls.DialWithDialer(&d, "tcp", addr, &tls.Config{InsecureSkipVerify: false})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("ami connect: %w", err)
	}
	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ami set deadline: %w", err)
	}
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	c := &ReadWriter{conn: conn, rw: rw}

	// Read the Asterisk manager greeting before authentication.
	greeting, err := c.readFrame()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ami read greeting: %w", err)
	}
	_ = greeting // The greeting must not be confused with a Login response.

	if err := c.login(cfg.Username, cfg.Secret); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ami login: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ami clear deadline: %w", err)
	}
	return c, nil
}

func (c *ReadWriter) login(username, secret string) error {
	resp, err := c.sendAction(map[string]string{
		"Action":   "Login",
		"Username": username,
		"Secret":   secret,
		"Events":   "off",
	})
	if err != nil {
		return err
	}
	if resp.Headers["Response"] != "Success" {
		return errors.New("ami authentication failed: " + resp.Headers["Message"])
	}
	return nil
}

// SendOriginate sends an Originate action and returns the response frame.
func (c *ReadWriter) SendOriginate(actionID, channel, context, exten, callerID string, timeout int, variables map[string]string) (*Frame, error) {
	headers := map[string]string{
		"Action":    "Originate",
		"ActionID":  actionID,
		"Channel":   channel,
		"Context":   context,
		"Exten":     exten,
		"Priority":  "1",
		"Timeout":   fmt.Sprintf("%d", timeout),
		"CallerID":  callerID,
		"Async":     "true",
	}
	return c.sendActionOriginate(headers, variables)
}

// SendAction sends a generic action and returns the response frame.
func (c *ReadWriter) SendAction(headers map[string]string) (*Frame, error) {
	return c.sendAction(headers)
}

// ReadFrame reads a complete AMI frame (headers terminated by a blank line).
// It handles CRLF separators and fragmented TCP packets.
func (c *ReadWriter) ReadFrame() (*Frame, error) {
	return c.readFrame()
}

// Logoff sends Logoff and closes the connection.
func (c *ReadWriter) Logoff() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.sendAction(map[string]string{"Action": "Logoff"})
	return c.Close()
}

// Close closes the AMI socket.
func (c *ReadWriter) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

func (c *ReadWriter) sendAction(headers map[string]string) (*Frame, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("ami connection closed")
	}
	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	if _, err := c.rw.WriteString(sb.String()); err != nil {
		return nil, fmt.Errorf("ami write: %w", err)
	}
	if err := c.rw.Flush(); err != nil {
		return nil, fmt.Errorf("ami flush: %w", err)
	}
	return c.readFrame()
}

func (c *ReadWriter) sendActionOriginate(headers map[string]string, variables map[string]string) (*Frame, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("ami connection closed")
	}
	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	for k, v := range variables {
		sb.WriteString("Variable: ")
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	if _, err := c.rw.WriteString(sb.String()); err != nil {
		return nil, fmt.Errorf("ami write: %w", err)
	}
	if err := c.rw.Flush(); err != nil {
		return nil, fmt.Errorf("ami flush: %w", err)
	}
	return c.readFrame()
}

func (c *ReadWriter) readFrame() (*Frame, error) {
	reader := c.rw.Reader
	frame := &Frame{Headers: make(map[string]string)}
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(lines) == 0 {
				return nil, errors.New("ami connection closed")
			}
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("ami read: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		lines = append(lines, line)
		if line == "" {
			break
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("ami empty frame")
	}
	first := true
	for _, line := range lines {
		if line == "" {
			continue
		}
		if first {
			idx := strings.Index(line, ":")
			if idx >= 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				frame.Headers[key] = val
			} else {
				frame.Headers["Greeting"] = line
			}
			first = false
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		frame.Headers[key] = val
	}
	frame.Raw = strings.Join(lines, "\r\n")
	return frame, nil
}

// IsSuccess reports whether a frame indicates success.
func IsSuccess(f *Frame) bool {
	if f == nil {
		return false
	}
	return f.Headers["Response"] == "Success"
}

package ami

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeAMI mimics an Asterisk AMI server for testing.
type fakeAMI struct {
	listener net.Listener
	port     int
	authOK   bool
	greeting string
	responses map[string]func(*Frame) *Frame // actionID -> response builder
	lastAction map[string]string
}

func startFakeAMI(t *testing.T, authOK bool) *fakeAMI {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeAMI{listener: ln, port: ln.Addr().(*net.TCPAddr).Port, authOK: authOK,
		greeting: "Asterisk Call Manager/20.0.0",
		responses: make(map[string]func(*Frame) *Frame),
		lastAction: make(map[string]string),
	}
	go f.serve()
	t.Cleanup(f.close)
	return f
}

func (f *fakeAMI) close() {
	_ = f.listener.Close()
}

func (f *fakeAMI) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeAMI) handle(conn net.Conn) {
	defer conn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	// Send greeting.
	greeting := f.greeting + "\r\n\r\n"
	if _, err := rw.WriteString(greeting); err != nil {
		return
	}
	rw.Flush()
	for {
		frame, err := readFrameRW(rw)
		if err != nil {
			if err != io.EOF {
				// ignore
			}
			return
		}
		action := frame.Headers["Action"]
		f.lastAction[action] = frame.Headers["ActionID"]
		if action == "Login" {
			resp := &Frame{Headers: map[string]string{}}
			if f.authOK {
				resp.Headers["Response"] = "Success"
				resp.Headers["Message"] = "Authentication accepted"
			} else {
				resp.Headers["Response"] = "Error"
				resp.Headers["Message"] = "Authentication failed"
			}
			writeFrameRW(rw, resp)
			continue
		}
		if action == "Originate" {
			resp := &Frame{Headers: map[string]string{}}
			actionID := frame.Headers["ActionID"]
			if builder, ok := f.responses[actionID]; ok {
				resp = builder(frame)
			} else {
				resp.Headers["Response"] = "Success"
				resp.Headers["Message"] = "Originate queued"
			}
			writeFrameRW(rw, resp)
			continue
		}
		if action == "Logoff" {
			writeFrameRW(rw, &Frame{Headers: map[string]string{"Response": "Goodbye"}})
			return
		}
		// Unknown action: generic success.
		writeFrameRW(rw, &Frame{Headers: map[string]string{"Response": "Success"}})
	}
}

func readFrameRW(rw *bufio.ReadWriter) (*Frame, error) {
	reader := rw.Reader
	frame := &Frame{Headers: map[string]string{}}
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(lines) == 0 {
				return nil, io.EOF
			}
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		lines = append(lines, line)
		if line == "" {
			break
		}
	}
	if len(lines) == 0 {
		return nil, io.EOF
	}
	first := true
	for _, line := range lines {
		if line == "" {
			continue
		}
		if first {
			idx := strings.Index(line, ":")
			if idx >= 0 {
				frame.Headers[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
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
		frame.Headers[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	frame.Raw = strings.Join(lines, "\r\n")
	return frame, nil
}

func writeFrameRW(rw *bufio.ReadWriter, f *Frame) error {
	var sb strings.Builder
	for k, v := range f.Headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	_, err := rw.WriteString(sb.String())
	if err != nil {
		return err
	}
	return rw.Flush()
}

func TestDialLoginSuccess(t *testing.T) {
	f := startFakeAMI(t, true)
	c, err := Dial(Config{Host: "127.0.0.1", Port: f.port, Username: "u", Secret: "s", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	return
}

func TestDialLoginFailure(t *testing.T) {
	f := startFakeAMI(t, false)
	_, err := Dial(Config{Host: "127.0.0.1", Port: f.port, Username: "u", Secret: "s", Timeout: 2 * time.Second})
	if err == nil {
		t.Fatal("expected login failure")
	}
}

func TestDialConnectionRefused(t *testing.T) {
	_, err := Dial(Config{Host: "127.0.0.1", Port: 1, Username: "u", Secret: "s", Timeout: 200 * time.Millisecond})
	if err == nil {
		t.Fatal("expected connection refused error")
	}
}

func TestOriginateSuccess(t *testing.T) {
	f := startFakeAMI(t, true)
	c, err := Dial(Config{Host: "127.0.0.1", Port: f.port, Username: "u", Secret: "s", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	resp, err := c.SendOriginate("req-1", "PJSIP/trunk/+989121234567", "voiceotp-playback", "s", "VoiceOTP", 30000, map[string]string{"VOICEOTP_CODE": "004829"})
	if err != nil {
		t.Fatalf("originate: %v", err)
	}
	if !IsSuccess(resp) {
		t.Fatalf("expected success, got %v", resp.Headers)
	}
	if f.lastAction["Originate"] != "req-1" {
		t.Fatalf("actionID mismatch: %s", f.lastAction["Originate"])
	}
}

func TestOriginateRejected(t *testing.T) {
	f := startFakeAMI(t, true)
	f.responses["req-reject"] = func(frame *Frame) *Frame {
		return &Frame{Headers: map[string]string{"Response": "Error", "Message": "Channel not found"}}
	}
	c, err := Dial(Config{Host: "127.0.0.1", Port: f.port, Username: "u", Secret: "s", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	resp, err := c.SendOriginate("req-reject", "PJSIP/trunk/+989121234567", "voiceotp-playback", "s", "VoiceOTP", 30000, nil)
	if err != nil {
		t.Fatalf("originate: %v", err)
	}
	if IsSuccess(resp) {
		t.Fatal("expected reject")
	}
}

func TestMalformedResponse(t *testing.T) {
	// A server that sends garbage then closes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, _ := ln.Accept()
		defer conn.Close()
		conn.Write([]byte("not an ami frame\r\nno headers\r\n\r\n"))
	}()
	_, err = Dial(Config{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "u", Secret: "s", Timeout: 2 * time.Second})
	if err == nil {
		t.Fatal("expected malformed response error")
	}
}

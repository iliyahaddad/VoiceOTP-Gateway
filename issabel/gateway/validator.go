package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// OTP digits only, configurable length.
var otpRe *regexp.Regexp

func init() {
	_ = initOTPRe(4, 8)
}

// initOTPRe builds the OTP regex from configured length bounds.
func initOTPRe(min, max int) error {
	if min <= 0 || max < min {
		return errors.New("invalid OTP length bounds")
	}
	re, err := regexp.Compile(fmt.Sprintf("^[0-9]{%d,%d}$", min, max))
	if err != nil {
		return err
	}
	otpRe = re
	return nil
}

// ValidateOTP checks that otp is a string of digits within configured bounds.
// It never converts the OTP to a numeric value.
func ValidateOTP(otp string) bool {
	if otpRe == nil {
		return false
	}
	return otpRe.MatchString(otp)
}

// phoneRe validates an E.164-style international number: leading '+', then digits.
var phoneRe = regexp.MustCompile(`^\+[0-9]{7,15}$`)

// ValidateNumber checks that the number is an E.164-style international number.
func ValidateNumber(number string) bool {
	return phoneRe.MatchString(number)
}

// NormalizeNumber strips whitespace and returns the number unchanged if valid.
// It rejects anything that could be interpreted as a SIP URI, channel name or
// dialplan expression.
func NormalizeNumber(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("invalid UTF-8 in number")
	}
	number := strings.TrimSpace(raw)
	if !ValidateNumber(number) {
		return "", errors.New("invalid telephone number")
	}
	return number, nil
}

// RequestBody is the decoded API request.
type RequestBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	OTP      string `json:"otp"`
	Number   string `json:"number"`
}

// ValidateRequest checks all four required fields.
func ValidateRequest(body RequestBody, cfg *APIConfig) error {
	if body.Username == "" {
		return errors.New("username is required")
	}
	if body.Password == "" {
		return errors.New("password is required")
	}
	if body.OTP == "" {
		return errors.New("otp is required")
	}
	if body.Number == "" {
		return errors.New("number is required")
	}
	if !ValidateOTP(body.OTP) {
		return errors.New("otp must be a string of digits within configured length")
	}
	if _, err := NormalizeNumber(body.Number); err != nil {
		return err
	}
	return nil
}

// GenerateRequestID returns a unique request identifier.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a smaller random source; still unique enough for correlation.
		b = make([]byte, 8)
		if _, rerr := rand.Read(b); rerr != nil {
			return "req-" + time.Now().UTC().Format("20060102150405.000000000")
		}
	}
	return fmt.Sprintf("req-%x", b)
}

// netIPFromCIDR parses a CIDR into network and mask.
func parseCIDR(cidr string) (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return ipnet, nil
}

// IPAllowed checks whether ip is within any of the allowed CIDRs.
func IPAllowed(ip net.IP, cidrs []string) bool {
	if ip == nil {
		return false
	}
	for _, cidr := range cidrs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// MaskNumber partially masks a telephone number for logging.
func MaskNumber(number string) string {
	if len(number) <= 4 {
		return "****"
	}
	return "****" + number[len(number)-4:]
}

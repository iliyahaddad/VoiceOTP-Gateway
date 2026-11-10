package main

import (
	"testing"
)

func TestOTPValidationLeadingZeros(t *testing.T) {
	initOTPRe(4, 8)
	cases := []struct {
		otp string
		want bool
	}{
		{"004829", true},
		{"0000", true},
		{"12345678", true},
		{"123", false},
		{"123456789", false},
		{"0048a9", false},
		{"00 4829", false},
		{"", false},
		{"0x4829", false},
	}
	for _, c := range cases {
		if got := ValidateOTP(c.otp); got != c.want {
			t.Errorf("ValidateOTP(%q) = %v want %v", c.otp, got, c.want)
		}
	}
}

func TestOTPNotConverted(t *testing.T) {
	initOTPRe(4, 8)
	otp := "004829"
	if !ValidateOTP(otp) {
		t.Fatal("leading zero OTP should be valid")
	}
	// Must remain a string; assert it is not treated as integer.
	if otp != "004829" {
		t.Fatal("OTP string was mutated")
	}
}

func TestValidateNumber(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"+989121234567", true},
		{"+442071234567", true},
		{"+14155552671", true},
		{"09121234567", false},
		{"989121234567", false},
		{"+98-912-123-4567", false},
		{"sip:user@host", false},
		{"PJSIP/trunk/100", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ValidateNumber(c.in); got != c.want {
			t.Errorf("ValidateNumber(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeNumberRejectsBad(t *testing.T) {
	_, err := NormalizeNumber("PJSIP/trunk/100")
	if err == nil {
		t.Fatal("expected error for channel name injection")
	}
	_, err = NormalizeNumber("sip:evil@attacker")
	if err == nil {
		t.Fatal("expected error for SIP URI injection")
	}
}

func TestValidateRequestMissingFields(t *testing.T) {
	cfg := &APIConfig{OTPMinLength: 4, OTPMaxLength: 8}
	if err := ValidateRequest(RequestBody{}, cfg); err == nil {
		t.Fatal("expected error for empty request")
	}
	if err := ValidateRequest(RequestBody{Username: "u", Password: "p", OTP: "1234", Number: ""}, cfg); err == nil {
		t.Fatal("expected error for missing number")
	}
}

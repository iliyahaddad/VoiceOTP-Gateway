package main

import (
	"net"
	"testing"
	"time"
)

func TestCredentialsAuth(t *testing.T) {
	c := NewCredentials("user", "pass")
	if !c.Auth("user", "pass") {
		t.Fatal("valid creds rejected")
	}
	if c.Auth("user", "wrong") {
		t.Fatal("wrong password accepted")
	}
	if c.Auth("wrong", "pass") {
		t.Fatal("wrong user accepted")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	if !constantTimeCompare("abc", "abc") {
		t.Fatal("equal strings should compare true")
	}
	if constantTimeCompare("abc", "abd") {
		t.Fatal("different strings should compare false")
	}
	if constantTimeCompare("abc", "") {
		t.Fatal("non-empty vs empty should be false")
	}
}

func TestRateLimiter(t *testing.T) {
	r := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !r.Allow("k") {
			t.Fatal("expected allow")
		}
	}
	if r.Allow("k") {
		t.Fatal("expected rate limit")
	}
	if !r.Allow("k2") {
		t.Fatal("expected allow for other key")
	}
	r.prune()
}

func TestIdempotencyStore(t *testing.T) {
	s := NewIdempotencyStore(time.Minute)
	if s.Seen("k1") {
		t.Fatal("new key seen")
	}
	if !s.Seen("k1") {
		t.Fatal("repeat key not seen")
	}
	if s.Seen("k2") {
		t.Fatal("other new key seen")
	}
	s.prune()
}

func TestCallSlot(t *testing.T) {
	s := NewCallSlot(2)
	if !s.Acquire() || !s.Acquire() {
		t.Fatal("expected two slots")
	}
	if s.TryAcquire() {
		t.Fatal("expected slot full")
	}
	s.Release()
	if !s.Acquire() {
		t.Fatal("expected slot after release")
	}
	s.Release()
	s.Release()
}

func TestIPAllowed(t *testing.T) {
	cidrs := []string{"127.0.0.1/32", "10.0.0.0/8"}
	if !IPAllowed(net.ParseIP("127.0.0.1"), cidrs) {
		t.Fatal("loopback should be allowed")
	}
	if !IPAllowed(net.ParseIP("10.1.2.3"), cidrs) {
		t.Fatal("10.x should be allowed")
	}
	if IPAllowed(net.ParseIP("8.8.8.8"), cidrs) {
		t.Fatal("public IP should not be allowed")
	}
	if IPAllowed(nil, cidrs) {
		t.Fatal("nil IP should not be allowed")
	}
}

func TestMaskNumber(t *testing.T) {
	if MaskNumber("+989121234567") != "****4567" {
		t.Fatal("mask wrong")
	}
	if MaskNumber("123") != "****" {
		t.Fatal("short number mask wrong")
	}
}

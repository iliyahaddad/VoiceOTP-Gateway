package main

import (
	"crypto/subtle"
	"sync"
	"time"
)

// constantTimeCompare performs a constant-time string comparison.
func constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Credentials holds API username/password for constant-time comparison.
type Credentials struct {
	username string
	password string
}

func NewCredentials(username, password string) *Credentials {
	return &Credentials{username: username, password: password}
}

// Auth checks the supplied credentials in constant time.
func (c *Credentials) Auth(username, password string) bool {
	if subtle.ConstantTimeCompare([]byte(username), []byte(c.username)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(c.password)) == 1
}

// RateLimiter implements a token-bucket style fixed-window limiter with bounded memory.
type RateLimiter struct {
	mu     sync.Mutex
	limits map[string]*rateEntry
	window time.Duration
	maxReq int
}

// rateEntry tracks hits within a fixed time window.
type rateEntry struct {
	count int
	reset time.Time
}

func NewRateLimiter(maxReq int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*rateEntry),
		window: window,
		maxReq: maxReq,
	}
}

// pruneLoop periodically prunes expired rate-limit windows.
func (r *RateLimiter) pruneLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.prune()
	}
}

// Allow returns true if the request is permitted for the given key.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	e, ok := r.limits[key]
	if !ok || now.After(e.reset) {
		r.limits[key] = &rateEntry{count: 1, reset: now.Add(r.window)}
		return true
	}
	if e.count >= r.maxReq {
		return false
	}
	e.count++
	return true
}

// prune removes expired windows to bound memory. Call periodically.
func (r *RateLimiter) prune() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for k, e := range r.limits {
		if now.After(e.reset) {
			delete(r.limits, k)
		}
	}
}

// IdempotencyStore tracks recently seen idempotency keys with bounded memory.
type IdempotencyStore struct {
	mu  sync.Mutex
	keys map[string]time.Time
	ttl  time.Duration
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{keys: make(map[string]time.Time), ttl: ttl}
}

// pruneLoop periodically prunes expired idempotency keys.
func (s *IdempotencyStore) pruneLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.prune()
	}
}

// Seen returns true if the key was already recorded; false and records it otherwise.
func (s *IdempotencyStore) Seen(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if t, ok := s.keys[key]; ok && now.Before(t) {
		return true
	}
	s.keys[key] = now.Add(s.ttl)
	return false
}

// prune removes expired keys.
func (s *IdempotencyStore) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, t := range s.keys {
		if now.After(t) {
			delete(s.keys, k)
		}
	}
}

// CallSlot bounds concurrent outbound calls.
type CallSlot struct {
	mu     sync.Mutex
	active int
	max    int
	cond   *sync.Cond
}

func NewCallSlot(max int) *CallSlot {
	s := &CallSlot{active: 0, max: max}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Acquire blocks until a call slot is available. Returns false on failure.
func (s *CallSlot) Acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.active >= s.max {
		s.cond.Wait()
	}
	if s.active >= s.max {
		return false
	}
	s.active++
	return true
}

// TryAcquire acquires a slot without blocking. Returns false if none available.
func (s *CallSlot) TryAcquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active >= s.max {
		return false
	}
	s.active++
	return true
}

// Release frees a call slot.
func (s *CallSlot) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active > 0 {
		s.active--
	}
	s.cond.Signal()
}

// Active returns the number of active calls.
func (s *CallSlot) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

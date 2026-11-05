package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	failCount int
	blockedAt time.Time
}

type SecurityGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

func NewSecurityGuard() *SecurityGuard {
	sg := &SecurityGuard{
		attempts: make(map[string]*loginAttempt),
	}
	// 1시간마다 오래된 IP 기록 정리
	go func() {
		for range time.Tick(1 * time.Hour) {
			sg.cleanup()
		}
	}()
	return sg
}

func (sg *SecurityGuard) getIP(r *http.Request) string {
	// Caddy의 X-Forwarded-For 우선 확인
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// CheckBlocked : [2번 전략] 5회 이상 실패 시 15분 차단 검사
func (sg *SecurityGuard) CheckBlocked(r *http.Request) (bool, time.Duration) {
	ip := sg.getIP(r)
	sg.mu.Lock()
	defer sg.mu.Unlock()

	att, exists := sg.attempts[ip]
	if !exists {
		return false, 0
	}

	if att.failCount >= 5 {
		remaining := 15*time.Minute - time.Since(att.blockedAt)
		if remaining > 0 {
			return true, remaining
		}
		// 15분 지났으면 초기화
		delete(sg.attempts, ip)
	}
	return false, 0
}

// RecordFailure : [1번 전략] 틀릴 때마다 점진적 지연 (1초, 2초, 4초...)
func (sg *SecurityGuard) RecordFailure(r *http.Request) {
	ip := sg.getIP(r)
	sg.mu.Lock()
	att, exists := sg.attempts[ip]
	if !exists {
		att = &loginAttempt{}
		sg.attempts[ip] = att
	}
	att.failCount++
	currentFail := att.failCount
	if currentFail >= 5 {
		att.blockedAt = time.Now()
	}
	sg.mu.Unlock()

	// 점진적 지연: 1회=1초, 2회=2초, 3회=3초, 4회 이상=5초
	delay := time.Duration(currentFail) * time.Second
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	time.Sleep(delay)
}

// RecordSuccess : 로그인 성공 시 실패 기록 초기화
func (sg *SecurityGuard) RecordSuccess(r *http.Request) {
	ip := sg.getIP(r)
	sg.mu.Lock()
	delete(sg.attempts, ip)
	sg.mu.Unlock()
}

// SafeCompare : [3번 전략] 타이밍 공격 방지 (상수 시간 문자열 비교)
func SafeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ValidateTOTP validates a standard Google Authenticator 6-digit TOTP token (RFC 6238)
func ValidateTOTP(secret, passcode string) bool {
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	if secret == "" || len(passcode) != 6 {
		return false
	}

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return false
		}
	}

	currentStep := time.Now().Unix() / 30
	// 허용 오차: 직전(-1), 현재(0), 직후(+1) 30초 단위
	for i := int64(-1); i <= 1; i++ {
		step := currentStep + i
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(step))

		mac := hmac.New(sha1.New, key)
		mac.Write(buf)
		sum := mac.Sum(nil)

		offset := sum[len(sum)-1] & 0x0f
		code := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1000000
		formattedCode := fmt.Sprintf("%06d", code)
		if formattedCode == passcode {
			return true
		}
	}
	return false
}

func (sg *SecurityGuard) cleanup() {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	now := time.Now()
	for ip, att := range sg.attempts {
		if now.Sub(att.blockedAt) > 1*time.Hour {
			delete(sg.attempts, ip)
		}
	}
}

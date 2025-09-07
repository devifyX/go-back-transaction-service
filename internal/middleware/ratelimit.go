package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type LimiterStore struct {
	mu    sync.Mutex
	ip    map[string]*entry
	user  map[string]*entry
	ipR   rate.Limit
	userR rate.Limit
	burst int
}

type entry struct {
	lim  *rate.Limiter
	last time.Time
}

func NewLimiterStore(ipPerMin, userPerMin int) *LimiterStore {
	return &LimiterStore{
		ip:    make(map[string]*entry),
		user:  make(map[string]*entry),
		ipR:   rate.Limit(float64(ipPerMin) / 60.0),
		userR: rate.Limit(float64(userPerMin) / 60.0),
		burst: 20,
	}
}

func (s *LimiterStore) get(lmap map[string]*entry, key string, r rate.Limit) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := lmap[key]; ok {
		e.last = time.Now()
		return e.lim
	}
	lim := rate.NewLimiter(r, s.burst)
	lmap[key] = &entry{lim: lim, last: time.Now()}
	return lim
}

func clientIP(r *http.Request) string {
	// Best effort: honor X-Forwarded-For if behind proxy
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// RateLimit enforces per-IP and per-user (via X-User-ID) limits.
func (s *LimiterStore) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ipLimiter := s.get(s.ip, ip, s.ipR)
		if !ipLimiter.Allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limit exceeded (ip)"))
			return
		}
		if uid := strings.TrimSpace(r.Header.Get("X-User-ID")); uid != "" {
			userLimiter := s.get(s.user, uid, s.userR)
			if !userLimiter.Allow() {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("rate limit exceeded (user)"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

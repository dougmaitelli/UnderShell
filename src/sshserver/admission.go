package sshserver

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type AdmissionConfig struct {
	MaxConnections       int
	MaxConnectionsPerIP  int
	MaxSessions          int
	HandshakesPerMinute  int
	RegistrationsPerHour int
}

type AdmissionMetrics struct {
	ConnectionsActive     int64
	ConnectionsAccepted   uint64
	ConnectionsRejected   uint64
	HandshakesRejected    uint64
	SessionsActive        int64
	SessionsAccepted      uint64
	SessionsRejected      uint64
	RegistrationsAllowed  uint64
	RegistrationsRejected uint64
}

type fixedWindowEntry struct {
	started time.Time
	seen    time.Time
	count   int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	entries map[string]fixedWindowEntry
}

func newFixedWindowLimiter(window time.Duration, limit int) fixedWindowLimiter {
	return fixedWindowLimiter{window: window, limit: limit, entries: make(map[string]fixedWindowEntry)}
}

func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	if l.limit <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		entry = fixedWindowEntry{started: now}
	}
	entry.seen = now
	if entry.count >= l.limit {
		l.entries[key] = entry
		return false
	}
	entry.count++
	l.entries[key] = entry
	if len(l.entries) > 1024 {
		for candidate, value := range l.entries {
			if now.Sub(value.seen) >= 2*l.window {
				delete(l.entries, candidate)
			}
		}
	}
	return true
}

type admissionController struct {
	config          AdmissionConfig
	mu              sync.Mutex
	connectionsByIP map[string]int
	connections     int
	sessions        int
	handshakes      fixedWindowLimiter
	registrations   fixedWindowLimiter

	connectionsActive     atomic.Int64
	connectionsAccepted   atomic.Uint64
	connectionsRejected   atomic.Uint64
	handshakesRejected    atomic.Uint64
	sessionsActive        atomic.Int64
	sessionsAccepted      atomic.Uint64
	sessionsRejected      atomic.Uint64
	registrationsAllowed  atomic.Uint64
	registrationsRejected atomic.Uint64
}

func newAdmissionController(config AdmissionConfig) *admissionController {
	return &admissionController{
		config: config, connectionsByIP: make(map[string]int),
		handshakes:    newFixedWindowLimiter(time.Minute, config.HandshakesPerMinute),
		registrations: newFixedWindowLimiter(time.Hour, config.RegistrationsPerHour),
	}
}

func (a *admissionController) acquireConnection(ip string, now time.Time) bool {
	if !a.handshakes.allow(ip, now) {
		a.handshakesRejected.Add(1)
		a.connectionsRejected.Add(1)
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.connections >= a.config.MaxConnections ||
		a.connectionsByIP[ip] >= a.config.MaxConnectionsPerIP {
		a.connectionsRejected.Add(1)
		return false
	}
	a.connections++
	a.connectionsByIP[ip]++
	a.connectionsActive.Add(1)
	a.connectionsAccepted.Add(1)
	return true
}

func (a *admissionController) releaseConnection(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.connectionsByIP[ip] == 0 {
		return
	}
	a.connections--
	a.connectionsByIP[ip]--
	if a.connectionsByIP[ip] == 0 {
		delete(a.connectionsByIP, ip)
	}
	a.connectionsActive.Add(-1)
}

func (a *admissionController) acquireSession() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions >= a.config.MaxSessions {
		a.sessionsRejected.Add(1)
		return false
	}
	a.sessions++
	a.sessionsActive.Add(1)
	a.sessionsAccepted.Add(1)
	return true
}

func (a *admissionController) releaseSession() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions == 0 {
		return
	}
	a.sessions--
	a.sessionsActive.Add(-1)
}

func (a *admissionController) AllowRegistration(ip string) bool {
	if a.registrations.allow(ip, time.Now()) {
		a.registrationsAllowed.Add(1)
		return true
	}
	a.registrationsRejected.Add(1)
	return false
}

func (a *admissionController) metrics() AdmissionMetrics {
	return AdmissionMetrics{
		ConnectionsActive: a.connectionsActive.Load(), ConnectionsAccepted: a.connectionsAccepted.Load(),
		ConnectionsRejected: a.connectionsRejected.Load(), HandshakesRejected: a.handshakesRejected.Load(),
		SessionsActive: a.sessionsActive.Load(), SessionsAccepted: a.sessionsAccepted.Load(),
		SessionsRejected: a.sessionsRejected.Load(), RegistrationsAllowed: a.registrationsAllowed.Load(),
		RegistrationsRejected: a.registrationsRejected.Load(),
	}
}

type admittedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *admittedConn) Close() error {
	c.once.Do(c.release)
	return c.Conn.Close()
}

func remoteIP(address net.Addr) string {
	if address == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(address.String())
	if err == nil {
		return host
	}
	return address.String()
}

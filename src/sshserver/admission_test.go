package sshserver

import (
	"testing"
	"time"
)

func TestAdmissionLimitsConnectionsSessionsAndRegistrations(t *testing.T) {
	controller := newAdmissionController(AdmissionConfig{
		MaxConnections: 2, MaxConnectionsPerIP: 1, MaxSessions: 1,
		HandshakesPerMinute: 10, RegistrationsPerHour: 1,
	})
	now := time.Now()
	if !controller.acquireConnection("192.0.2.1", now) {
		t.Fatal("first connection rejected")
	}
	if controller.acquireConnection("192.0.2.1", now) {
		t.Fatal("per-IP connection limit was not enforced")
	}
	if !controller.acquireConnection("192.0.2.2", now) {
		t.Fatal("second IP connection rejected")
	}
	if controller.acquireConnection("192.0.2.3", now) {
		t.Fatal("global connection limit was not enforced")
	}
	controller.releaseConnection("192.0.2.1")
	if !controller.acquireConnection("192.0.2.3", now) {
		t.Fatal("released connection capacity was not reusable")
	}

	if !controller.acquireSession() || controller.acquireSession() {
		t.Fatal("session limit was not enforced")
	}
	controller.releaseSession()
	if !controller.acquireSession() {
		t.Fatal("released session capacity was not reusable")
	}

	if !controller.AllowRegistration("192.0.2.4") ||
		controller.AllowRegistration("192.0.2.4") {
		t.Fatal("registration limit was not enforced")
	}
	metrics := controller.metrics()
	if metrics.ConnectionsActive != 2 || metrics.ConnectionsRejected != 2 ||
		metrics.SessionsRejected != 1 || metrics.RegistrationsRejected != 1 {
		t.Fatalf("unexpected admission metrics: %#v", metrics)
	}
}

func TestHandshakeRateLimitResetsAfterWindow(t *testing.T) {
	controller := newAdmissionController(AdmissionConfig{
		MaxConnections: 10, MaxConnectionsPerIP: 10, MaxSessions: 10,
		HandshakesPerMinute: 1, RegistrationsPerHour: 1,
	})
	now := time.Now()
	if !controller.acquireConnection("192.0.2.1", now) {
		t.Fatal("first handshake rejected")
	}
	controller.releaseConnection("192.0.2.1")
	if controller.acquireConnection("192.0.2.1", now.Add(time.Second)) {
		t.Fatal("handshake rate limit was not enforced")
	}
	if !controller.acquireConnection("192.0.2.1", now.Add(time.Minute)) {
		t.Fatal("handshake window did not reset")
	}
}

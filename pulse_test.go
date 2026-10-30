package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSessionCookieEncryptionRoundTrip(t *testing.T) {
	SetCookieEncryptionKey("test-secret-key")
	orig := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "https://pulse.example.com/", nil)
	rec := httptest.NewRecorder()
	if err := SetSessionCookie(rec, req, orig); err != nil {
		t.Fatalf("SetSessionCookie failed: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Errorf("expected Secure cookie on https request")
	}
	if !cookies[0].HttpOnly {
		t.Errorf("expected HttpOnly cookie")
	}

	req2 := httptest.NewRequest(http.MethodGet, "https://pulse.example.com/api/status", nil)
	req2.AddCookie(cookies[0])
	decoded, err := GetSessionCookie(req2)
	if err != nil {
		t.Fatalf("GetSessionCookie failed: %v", err)
	}
	if decoded.AccessToken != orig.AccessToken || decoded.RefreshToken != orig.RefreshToken {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, orig)
	}
}

func TestOAuthStateNonceValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/auth-url", nil)
	rec := httptest.NewRecorder()
	state, err := SetOAuthStateCookie(rec, req)
	if err != nil {
		t.Fatalf("SetOAuthStateCookie failed: %v", err)
	}
	if len(state) < 16 {
		t.Fatalf("expected random state nonce, got %q", state)
	}

	cookies := rec.Result().Cookies()
	cbReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/oauth/callback?state="+state, nil)
	for _, c := range cookies {
		cbReq.AddCookie(c)
	}
	cbRec := httptest.NewRecorder()
	if !ValidateOAuthStateCookie(cbRec, cbReq) {
		t.Fatalf("ValidateOAuthStateCookie returned false for valid state")
	}

	tamperedReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/oauth/callback?state=tampered", nil)
	for _, c := range cookies {
		tamperedReq.AddCookie(c)
	}
	if ValidateOAuthStateCookie(httptest.NewRecorder(), tamperedReq) {
		t.Errorf("expected false for mismatched state")
	}
}

func TestEffectiveRedirectURI(t *testing.T) {
	svc := NewStatelessService("id", "secret", "http://localhost:8080/oauth/callback")
	srv := NewServer(svc, "http://localhost:8080/oauth/callback")

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/status", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "health.myhome.lan:8443")

	got := srv.effectiveRedirectURI(req)
	want := "https://health.myhome.lan:8443/oauth/callback"
	if got != want {
		t.Errorf("effectiveRedirectURI = %q, want %q", got, want)
	}

	customSvc := NewStatelessService("id", "secret", "http://localhost:9090/oauth/callback")
	customSrv := NewServer(customSvc, "http://localhost:9090/oauth/callback")
	gotCustom := customSrv.effectiveRedirectURI(req)
	if gotCustom != "http://localhost:9090/oauth/callback" {
		t.Errorf("effectiveRedirectURI with explicit custom REDIRECT_URL = %q, want http://localhost:9090/oauth/callback", gotCustom)
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	svc := NewStatelessService("id", "secret", "http://localhost:8080/oauth/callback")
	srv := NewServer(svc, "http://localhost:8080/oauth/callback")
	srv.SetPassword("supersecret")
	handler := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", rec.Code)
	}

	reqAuth := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	reqAuth.SetBasicAuth("pulse", "supersecret")
	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid basic auth password, got %d", recAuth.Code)
	}

	var status StatusResponse
	if err := json.Unmarshal(recAuth.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode StatusResponse: %v", err)
	}
	if !status.ClientConfigured {
		t.Errorf("expected ClientConfigured = true")
	}
	if status.RedirectURI == "" {
		t.Errorf("expected non-empty RedirectURI in StatusResponse")
	}
}

func TestParsingHelpers(t *testing.T) {
	if mins := parseDurationMinutes("3660s"); mins != 61 {
		t.Errorf("parseDurationMinutes(3660s) = %d, want 61", mins)
	}
	if val := parseAnyFloat("72.5"); val != 72.5 {
		t.Errorf("parseAnyFloat = %v, want 72.5", val)
	}
	civil := &CivilDateTime{Date: Date{Year: 2026, Month: 3, Day: 9}}
	if d := extractDate(civil, ""); d != "2026-03-09" {
		t.Errorf("extractDate = %q, want 2026-03-09", d)
	}
	if d := extractDate(nil, "2026-10-31T08:15:00Z"); d != "2026-10-31" {
		t.Errorf("extractDate fallback = %q, want 2026-10-31", d)
	}
}

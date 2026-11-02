package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

	dpSteps := DataPoint{
		Steps: &Steps{
			Interval: ObservationTimeInterval{
				CivilStartTime: &CivilDateTime{Date: Date{Year: 2026, Month: 5, Day: 10}},
			},
		},
	}
	if d := extractDataPointDate(dpSteps); d != "2026-05-10" {
		t.Errorf("extractDataPointDate(Steps) = %q, want 2026-05-10", d)
	}

	dpActiveEnergy := DataPoint{
		ActiveEnergyBurned: &ActiveEnergyBurned{
			Interval: ObservationTimeInterval{
				StartTime: "2026-04-12T12:00:00Z",
			},
		},
	}
	if d := extractDataPointDate(dpActiveEnergy); d != "2026-04-12" {
		t.Errorf("extractDataPointDate(ActiveEnergy) = %q, want 2026-04-12", d)
	}

	dpHRZone := DataPoint{
		TimeInHeartRateZone: &TimeInHeartRateZoneAPI{
			Interval: ObservationTimeInterval{
				StartTime: "2026-08-20T15:30:00Z",
			},
		},
	}
	if d := extractDataPointDate(dpHRZone); d != "2026-08-20" {
		t.Errorf("extractDataPointDate(TimeInHeartRateZone) = %q, want 2026-08-20", d)
	}
}

func TestLoadDotEnv(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	content := "# comment\nPULSE_TEST_ENV_VAR=\"hello-world\"\nexport PULSE_TEST_SECOND=42\n"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp .env: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("PULSE_TEST_ENV_VAR")
		_ = os.Unsetenv("PULSE_TEST_SECOND")
	})
	loadDotEnv(tmp)
	if got := os.Getenv("PULSE_TEST_ENV_VAR"); got != "hello-world" {
		t.Errorf("PULSE_TEST_ENV_VAR = %q, want hello-world", got)
	}
	if got := os.Getenv("PULSE_TEST_SECOND"); got != "42" {
		t.Errorf("PULSE_TEST_SECOND = %q, want 42", got)
	}
}

func TestMultiUserCacheIsolation(t *testing.T) {
	svc := NewStatelessService("id", "secret", "http://localhost:8080/oauth/callback")
	tokA := &oauth2.Token{AccessToken: "acc-a", RefreshToken: "ref-user-a"}
	tokB := &oauth2.Token{AccessToken: "acc-b", RefreshToken: "ref-user-b"}

	keyA := userCacheKey(tokA)
	keyB := userCacheKey(tokB)
	if keyA == keyB {
		t.Fatalf("expected distinct cache keys for different users, got %q", keyA)
	}

	svc.statsCache[keyA+":3650:overview:"] = cachedStatsEntry{resp: &StatsResponse{}, cachedAt: time.Now()}
	svc.statsCache[keyB+":3650:overview:"] = cachedStatsEntry{resp: &StatsResponse{}, cachedAt: time.Now()}

	svc.ClearUserCache(tokA)
	if _, ok := svc.statsCache[keyA+":3650:overview:"]; ok {
		t.Errorf("expected user A cache entry to be cleared")
	}
	if _, ok := svc.statsCache[keyB+":3650:overview:"]; !ok {
		t.Errorf("expected user B cache entry to remain intact")
	}
}

func TestFetchActivityPagesTargetDate(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		pageToken := r.URL.Query().Get("pageToken")

		var resp ListDataPointsResponse
		if pageToken == "" {
			resp.NextPageToken = "token-page-2"
			resp.DataPoints = []DataPoint{
				{
					Steps: &Steps{
						Count: "5000",
						Interval: ObservationTimeInterval{
							CivilStartTime: &CivilDateTime{Date: Date{Year: 2026, Month: 11, Day: 1}},
						},
					},
				},
			}
		} else if pageToken == "token-page-2" {
			resp.NextPageToken = "token-page-3"
			resp.DataPoints = []DataPoint{
				{
					Steps: &Steps{
						Count: "4200",
						Interval: ObservationTimeInterval{
							CivilStartTime: &CivilDateTime{Date: Date{Year: 2026, Month: 10, Day: 15}},
						},
					},
				},
			}
		} else {
			resp.DataPoints = []DataPoint{
				{
					Steps: &Steps{
						Count: "3000",
						Interval: ObservationTimeInterval{
							CivilStartTime: &CivilDateTime{Date: Date{Year: 2026, Month: 9, Day: 1}},
						},
					},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	svc := NewStatelessService("id", "secret", "http://localhost:8080/oauth/callback")

	// Custom round tripper redirecting health.googleapis.com to test server
	testClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			targetURL, _ := url.Parse(ts.URL + req.URL.Path + "?" + req.URL.RawQuery)
			newReq, _ := http.NewRequest(req.Method, targetURL.String(), req.Body)
			return http.DefaultTransport.RoundTrip(newReq)
		}),
	}

	// Requesting target date 2026-10-20 should stop after page 2 (which reaches 2026-10-15)
	dps, nextTok, err := svc.fetchActivityPages(testClient, "steps", "", "2026-10-20", 10)
	if err != nil {
		t.Fatalf("fetchActivityPages failed: %v", err)
	}
	if len(dps) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(dps))
	}
	if requestCount != 2 {
		t.Errorf("expected 2 page requests, got %d", requestCount)
	}
	if nextTok != "token-page-3" {
		t.Errorf("expected next token 'token-page-3', got %q", nextTok)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestMergeStatsResponses(t *testing.T) {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	threeDaysAgo := now.AddDate(0, 0, -3).Format("2006-01-02")
	tenDaysAgo := now.AddDate(0, 0, -10).Format("2006-01-02")

	existing := &StatsResponse{
		ActivityRecords: []ActivityRecord{
			{Date: threeDaysAgo, Steps: 5000},
			{Date: tenDaysAgo, Steps: 8000},
		},
		SleepSessions: []SleepSession{
			{ID: "session-old", StartTime: now.AddDate(0, 0, -5).Format(time.RFC3339), DurationMinutes: 400},
			{ID: "session-yesterday", StartTime: now.AddDate(0, 0, -1).Format(time.RFC3339), DurationMinutes: 420},
		},
		RHRRecords: []RHRRecord{
			{Date: threeDaysAgo, BeatsPerMinute: 62},
			{Date: tenDaysAgo, BeatsPerMinute: 65},
		},
	}

	fresh := &StatsResponse{
		ActivityRecords: []ActivityRecord{
			{Date: today, Steps: 3500},
			{Date: threeDaysAgo, Steps: 5500}, // updated value
		},
		SleepSessions: []SleepSession{
			{ID: "session-yesterday", StartTime: now.AddDate(0, 0, -1).Format(time.RFC3339), DurationMinutes: 450}, // updated duration
			{ID: "session-today", StartTime: now.Format(time.RFC3339), DurationMinutes: 480},                       // new session
		},
		RHRRecords: []RHRRecord{
			{Date: today, BeatsPerMinute: 60},
		},
	}

	// 1. Merge with no cutoff (days = 0)
	mergedAll := mergeStatsResponses(existing, fresh, 0)
	if len(mergedAll.ActivityRecords) != 3 {
		t.Fatalf("expected 3 activity records without cutoff, got %d", len(mergedAll.ActivityRecords))
	}
	if mergedAll.ActivityRecords[0].Date != today || mergedAll.ActivityRecords[0].Steps != 3500 {
		t.Errorf("expected newest today record first with 3500 steps, got %+v", mergedAll.ActivityRecords[0])
	}
	if mergedAll.ActivityRecords[1].Date != threeDaysAgo || mergedAll.ActivityRecords[1].Steps != 5500 {
		t.Errorf("expected updated 5500 steps for threeDaysAgo, got %+v", mergedAll.ActivityRecords[1])
	}
	if mergedAll.ActivityRecords[2].Date != tenDaysAgo {
		t.Errorf("expected tenDaysAgo record preserved, got %+v", mergedAll.ActivityRecords[2])
	}

	// Verify sleep sessions merged by ID and sorted descending
	if len(mergedAll.SleepSessions) != 3 {
		t.Fatalf("expected 3 sleep sessions, got %d", len(mergedAll.SleepSessions))
	}
	if mergedAll.SleepSessions[0].ID != "session-today" {
		t.Errorf("expected session-today first, got %s", mergedAll.SleepSessions[0].ID)
	}
	if mergedAll.SleepSessions[1].ID != "session-yesterday" || mergedAll.SleepSessions[1].DurationMinutes != 450 {
		t.Errorf("expected updated duration 450 for session-yesterday, got %+v", mergedAll.SleepSessions[1])
	}

	// 2. Merge with 7-day cutoff (days = 7)
	merged7d := mergeStatsResponses(existing, fresh, 7)
	if len(merged7d.ActivityRecords) != 2 {
		t.Fatalf("expected 2 activity records within 7 days, got %d", len(merged7d.ActivityRecords))
	}
	for _, r := range merged7d.ActivityRecords {
		if r.Date == tenDaysAgo {
			t.Errorf("tenDaysAgo record should have been trimmed by 7-day cutoff")
		}
	}
	if len(merged7d.RHRRecords) != 2 {
		t.Fatalf("expected 2 RHR records within 7 days, got %d", len(merged7d.RHRRecords))
	}

	// Verify nil safety
	nilMerged := mergeStatsResponses(nil, fresh, 0)
	if nilMerged != fresh {
		t.Errorf("expected fresh when existing is nil")
	}
	nilMerged2 := mergeStatsResponses(existing, nil, 0)
	if nilMerged2 != existing {
		t.Errorf("expected existing when fresh is nil")
	}
}

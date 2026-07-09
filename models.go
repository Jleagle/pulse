package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Session Cookie Configuration
const (
	SessionCookieName = "pulse_session"
	CookieMaxAge      = 30 * 24 * 60 * 60 // 30 days in seconds
)

// Sleep Session Data Access
type SleepSession struct {
	ID              string       `json:"id"`
	StartTime       string       `json:"start_time"`
	EndTime         string       `json:"end_time"`
	DurationMinutes int          `json:"duration_minutes"`
	MinutesAsleep   int          `json:"minutes_asleep"`
	MinutesAwake    int          `json:"minutes_awake"`
	SleepType       string       `json:"sleep_type"`
	Stages          []SleepStage `json:"stages,omitempty"`
}

type SleepStage struct {
	StageType       string `json:"stage_type"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	DurationMinutes int    `json:"duration_minutes"`
}

// Resting Heart Rate Data Access
type RHRRecord struct {
	Date           string `json:"date"`
	BeatsPerMinute int    `json:"beats_per_minute"`
}

// HRV Data Access
type HRVRecord struct {
	Date           string   `json:"date"`
	AvgHRVMs       float64  `json:"avg_hrv_ms"`
	Entropy        *float64 `json:"entropy"`
	DeepSleepRMSSD *float64 `json:"deep_sleep_rmssd"`
}

// Activity Data Access
type ActivityRecord struct {
	Date           string `json:"date"`
	Steps          int    `json:"steps"`
	CaloriesBurned int    `json:"calories_burned"`
	ActiveMinutes  int    `json:"active_minutes"`
}

// Stats Response for the frontend
type StatsResponse struct {
	SleepSessions         []SleepSession   `json:"sleep_sessions"`
	SleepNextPageToken    string           `json:"sleep_next_page_token,omitempty"`
	RHRRecords            []RHRRecord      `json:"rhr_records"`
	RHRNextPageToken      string           `json:"rhr_next_page_token,omitempty"`
	HRVRecords            []HRVRecord      `json:"hrv_records"`
	HRVNextPageToken      string           `json:"hrv_next_page_token,omitempty"`
	ActivityRecords       []ActivityRecord `json:"activity_records"`
	ActivityNextPageToken string           `json:"activity_next_page_token,omitempty"`
	MissingScopes         []string         `json:"missing_scopes,omitempty"`
}

// Status Response for the frontend
type StatusResponse struct {
	ClientConfigured bool   `json:"client_configured"`
	OAuthConnected   bool   `json:"oauth_connected"`
	UserEmail        string `json:"user_email"`
	UserName         string `json:"user_name"`
	UserPicture      string `json:"user_picture"`
}

// User Profile info from Google OAuth
type UserProfile struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Cookie Session Helpers
func SetSessionCookie(w http.ResponseWriter, token *oauth2.Token) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(tokenJSON)
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true, // Secure against XSS
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)
	return nil
}

func GetSessionCookie(r *http.Request) (*oauth2.Token, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, err
	}

	if cookie.Value == "" {
		return nil, fmt.Errorf("empty session cookie")
	}

	val := cookie.Value
	var tokenJSON []byte
	if strings.Contains(val, "=") {
		tokenJSON, err = base64.URLEncoding.DecodeString(val)
	} else {
		tokenJSON, err = base64.RawURLEncoding.DecodeString(val)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode session cookie: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check if token has expired and doesn't have a refresh token
	if !token.Valid() && token.RefreshToken == "" {
		return nil, fmt.Errorf("token expired and no refresh token available")
	}

	return &token, nil
}

func ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

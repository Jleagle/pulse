package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// Session Cookie Configuration
const (
	SessionCookieName    = "pulse_session"
	OAuthStateCookieName = "pulse_oauth_state"
	CookieMaxAge         = 30 * 24 * 60 * 60 // 30 days in seconds
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
	Date           string  `json:"date"`
	Steps          int     `json:"steps"`
	CaloriesBurned int     `json:"calories_burned"`
	BasalCalories  int     `json:"basal_calories"`
	ActiveMinutes  int     `json:"active_minutes"`
	DistanceKm     float64 `json:"distance_km"`
	Floors         int     `json:"floors"`
}

// Heart Rate Zones & VO2 Max Data Access
type HRZoneRecord struct {
	Date            string `json:"date"`
	LightMinutes    int    `json:"light_minutes"`
	ModerateMinutes int    `json:"moderate_minutes"`
	VigorousMinutes int    `json:"vigorous_minutes"`
	PeakMinutes     int    `json:"peak_minutes"`
}

type VO2MaxRecord struct {
	Date   string  `json:"date"`
	VO2Max float64 `json:"vo2_max"`
}

// Workouts & Exercise Route Data Access
type ExerciseSession struct {
	ID                string  `json:"id"`
	StartTime         string  `json:"start_time"`
	EndTime           string  `json:"end_time"`
	ExerciseType      string  `json:"exercise_type"`
	DisplayName       string  `json:"display_name"`
	DurationMinutes   int     `json:"duration_minutes"`
	CaloriesKcal      int     `json:"calories_kcal"`
	DistanceKm        float64 `json:"distance_km"`
	Steps             int     `json:"steps"`
	AvgHeartRate      int     `json:"avg_heart_rate"`
	ActiveZoneMinutes int     `json:"active_zone_minutes"`
	LightZoneMins     int     `json:"light_zone_mins"`
	ModerateZoneMins  int     `json:"moderate_zone_mins"`
	VigorousZoneMins  int     `json:"vigorous_zone_mins"`
	PeakZoneMins      int     `json:"peak_zone_mins"`
	HasGPS            bool    `json:"has_gps"`
}

type RoutePoint struct {
	Timestamp      string  `json:"timestamp"`
	ElapsedSeconds int     `json:"elapsed_seconds"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	AltitudeMeters float64 `json:"altitude_meters"`
	DistanceMeters float64 `json:"distance_meters"`
	HeartRateBpm   int     `json:"heart_rate_bpm"`
	PaceMinPerKm   float64 `json:"pace_min_per_km,omitempty"`
}

type ExerciseRouteResponse struct {
	ExerciseID string       `json:"exercise_id"`
	HasRoute   bool         `json:"has_route"`
	Points     []RoutePoint `json:"points"`
}

// Vitals & Body Data Access
type SpO2Record struct {
	Date       string  `json:"date"`
	AvgPercent float64 `json:"avg_percent"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
}

type RespiratoryRateRecord struct {
	Date             string  `json:"date"`
	BreathsPerMinute float64 `json:"breaths_per_minute"`
}

type SleepTempRecord struct {
	Date         string  `json:"date"`
	NightlyTempC float64 `json:"nightly_temp_c"`
	BaselineC    float64 `json:"baseline_c"`
	DeltaC       float64 `json:"delta_c"`
}

type BodyRecord struct {
	Date       string   `json:"date"`
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
}

// Nutrition & Hydration Data Access
type NutritionRecord struct {
	Date        string  `json:"date"`
	HydrationMl int     `json:"hydration_ml"`
	CaloriesIn  int     `json:"calories_in"`
	ProteinG    float64 `json:"protein_g"`
	CarbsG      float64 `json:"carbs_g"`
	FatG        float64 `json:"fat_g"`
	MealsLogged int     `json:"meals_logged"`
}

// Stats Response for the frontend
type StatsResponse struct {
	SleepSessions         []SleepSession          `json:"sleep_sessions"`
	SleepNextPageToken    string                  `json:"sleep_next_page_token,omitempty"`
	RHRRecords            []RHRRecord             `json:"rhr_records"`
	RHRNextPageToken      string                  `json:"rhr_next_page_token,omitempty"`
	HRVRecords            []HRVRecord             `json:"hrv_records"`
	HRVNextPageToken      string                  `json:"hrv_next_page_token,omitempty"`
	HRZoneRecords         []HRZoneRecord          `json:"hr_zone_records"`
	VO2MaxRecords         []VO2MaxRecord          `json:"vo2_max_records"`
	ActivityRecords       []ActivityRecord        `json:"activity_records"`
	ActivityNextPageToken string                  `json:"activity_next_page_token,omitempty"`
	ExerciseSessions      []ExerciseSession       `json:"exercise_sessions"`
	ExerciseNextPageToken string                  `json:"exercise_next_page_token,omitempty"`
	SpO2Records           []SpO2Record            `json:"spo2_records"`
	RespiratoryRecords    []RespiratoryRateRecord `json:"respiratory_records"`
	SleepTempRecords      []SleepTempRecord       `json:"sleep_temp_records"`
	BodyRecords           []BodyRecord            `json:"body_records"`
	NutritionRecords      []NutritionRecord       `json:"nutrition_records"`
	MissingScopes         []string                `json:"missing_scopes,omitempty"`
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
var (
	cookieKeyMu sync.RWMutex
	cookieKey   = func() []byte {
		b := make([]byte, 32)
		_, _ = io.ReadFull(rand.Reader, b)
		return b
	}()
)

// SetCookieEncryptionKey derives a 32-byte AES-256 key from the configured secret.
func SetCookieEncryptionKey(secret string) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return
	}
	sum := sha256.Sum256([]byte("pulse-cookie-key:" + secret))
	cookieKeyMu.Lock()
	cookieKey = sum[:]
	cookieKeyMu.Unlock()
}

func getCookieCipher() (cipher.AEAD, error) {
	cookieKeyMu.RLock()
	key := cookieKey
	cookieKeyMu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptCookiePayload(plaintext []byte) (string, error) {
	gcm, err := getCookieCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func decryptCookiePayload(encoded string) ([]byte, error) {
	var raw []byte
	var err error
	if strings.Contains(encoded, "=") {
		raw, err = base64.URLEncoding.DecodeString(encoded)
	} else {
		raw, err = base64.RawURLEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode session cookie: %w", err)
	}

	gcm, err := getCookieCipher()
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("encrypted cookie payload too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session cookie: %w", err)
	}
	return plaintext, nil
}

func SetSessionCookie(w http.ResponseWriter, token *oauth2.Token) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	encoded, err := encryptCookiePayload(tokenJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}
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

	tokenJSON, err := decryptCookiePayload(cookie.Value)
	if err != nil {
		return nil, err
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

func SetOAuthStateCookie(w http.ResponseWriter) (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return state, nil
}

func ValidateOAuthStateCookie(w http.ResponseWriter, r *http.Request) bool {
	stateParam := r.URL.Query().Get("state")
	cookie, err := r.Cookie(OAuthStateCookieName)
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if err != nil || cookie.Value == "" || stateParam == "" {
		return false
	}
	return cookie.Value == stateParam
}

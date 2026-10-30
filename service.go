package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// API Structs matching Google Health API v4 discovery doc
type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

type CivilDateTime struct {
	Date Date `json:"date"`
}

type ObservationTimeInterval struct {
	StartTime      string         `json:"startTime"`
	EndTime        string         `json:"endTime"`
	CivilStartTime *CivilDateTime `json:"civilStartTime,omitempty"`
}

type SessionTimeInterval struct {
	StartTime      string         `json:"startTime"`
	EndTime        string         `json:"endTime"`
	CivilStartTime *CivilDateTime `json:"civilStartTime,omitempty"`
}

type SleepStageAPI struct {
	Type      string `json:"type"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type SleepSummaryAPI struct {
	MinutesInSleepPeriod string `json:"minutesInSleepPeriod"`
	MinutesAsleep        string `json:"minutesAsleep"`
	MinutesAwake         string `json:"minutesAwake"`
}

type Sleep struct {
	Interval SessionTimeInterval `json:"interval"`
	Type     string              `json:"type"`
	Stages   []SleepStageAPI     `json:"stages"`
	Summary  *SleepSummaryAPI    `json:"summary"`
}

type DailyRestingHeartRate struct {
	Date           Date   `json:"date"`
	BeatsPerMinute string `json:"beatsPerMinute"`
}

type DailyHeartVariable struct {
	Date                                                       Date     `json:"date"`
	Entropy                                                    *float64 `json:"entropy"`
	AverageHeartRateVariabilityMilliseconds                    *float64 `json:"averageHeartRateVariabilityMilliseconds"`
	DeepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds *float64 `json:"deepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds"`
}

type Steps struct {
	Count    string                  `json:"count"`
	Interval ObservationTimeInterval `json:"interval"`
}

type ActiveEnergyBurned struct {
	Kcal     float64                 `json:"kcal"`
	Interval ObservationTimeInterval `json:"interval"`
}

type ActiveMinutesByActivityLevel struct {
	ActivityLevel string `json:"activityLevel"`
	ActiveMinutes string `json:"activeMinutes"`
}

type ActiveMinutes struct {
	Interval                     ObservationTimeInterval        `json:"interval"`
	ActiveMinutesByActivityLevel []ActiveMinutesByActivityLevel `json:"activeMinutesByActivityLevel"`
}

type SampleTimeAPI struct {
	PhysicalTime string         `json:"physicalTime"`
	CivilTime    *CivilDateTime `json:"civilTime,omitempty"`
}

type DistanceAPI struct {
	DistanceMillimeters interface{}             `json:"distanceMillimeters"`
	Millimeters         interface{}             `json:"millimeters"`
	Interval            ObservationTimeInterval `json:"interval"`
}

type FloorValue struct {
	Count string `json:"count"`
}

type FloorsAPI struct {
	Count    string                  `json:"count"`
	Values   []FloorValue            `json:"values"`
	Interval ObservationTimeInterval `json:"interval"`
}

type BasalEnergyBurnedAPI struct {
	Kcal     interface{}             `json:"kcal"`
	Interval ObservationTimeInterval `json:"interval"`
}

type TimeInHeartRateZoneAPI struct {
	Interval          ObservationTimeInterval `json:"interval"`
	StartTime         string                  `json:"startTime"`
	EndTime           string                  `json:"endTime"`
	HeartRateZoneType string                  `json:"heartRateZoneType"`
}

type DailyVO2MaxAPI struct {
	Date        Date        `json:"date"`
	VO2Max      interface{} `json:"vo2Max"`
	VO2MaxValue interface{} `json:"vo2MaxValue"`
}

type VO2MaxAPI struct {
	SampleTime  SampleTimeAPI `json:"sampleTime"`
	VO2MaxValue interface{}   `json:"vo2MaxValue"`
	VO2Max      interface{}   `json:"vo2Max"`
}

type HeartRateZoneDurationsAPI struct {
	LightTime    string `json:"lightTime"`
	ModerateTime string `json:"moderateTime"`
	VigorousTime string `json:"vigorousTime"`
	PeakTime     string `json:"peakTime"`
}

type ExerciseMetricsSummaryAPI struct {
	CaloriesKcal                   interface{}                `json:"caloriesKcal"`
	DistanceMillimeters            interface{}                `json:"distanceMillimeters"`
	Steps                          interface{}                `json:"steps"`
	AverageHeartRateBeatsPerMinute interface{}                `json:"averageHeartRateBeatsPerMinute"`
	ActiveZoneMinutes              interface{}                `json:"activeZoneMinutes"`
	HeartRateZoneDurations         *HeartRateZoneDurationsAPI `json:"heartRateZoneDurations,omitempty"`
}

type ExerciseAPI struct {
	Interval                       SessionTimeInterval        `json:"interval"`
	ExerciseType                   string                     `json:"exerciseType"`
	DisplayName                    string                     `json:"displayName"`
	CaloriesKcal                   interface{}                `json:"caloriesKcal"`
	DistanceMillimeters            interface{}                `json:"distanceMillimeters"`
	Steps                          interface{}                `json:"steps"`
	AverageHeartRateBeatsPerMinute interface{}                `json:"averageHeartRateBeatsPerMinute"`
	HeartRateZoneDurations         *HeartRateZoneDurationsAPI `json:"heartRateZoneDurations,omitempty"`
	MetricsSummary                 *ExerciseMetricsSummaryAPI `json:"metricsSummary,omitempty"`
}

type DailyOxygenSaturationAPI struct {
	Date                 Date        `json:"date"`
	AveragePercentage    interface{} `json:"averagePercentage"`
	LowerBoundPercentage interface{} `json:"lowerBoundPercentage"`
	UpperBoundPercentage interface{} `json:"upperBoundPercentage"`
}

type DailyRespiratoryRateAPI struct {
	Date             Date        `json:"date"`
	RateBpm          interface{} `json:"rateBpm"`
	BreathsPerMinute interface{} `json:"breathsPerMinute"`
}

type DailySleepTemperatureDerivationsAPI struct {
	Date                            Date        `json:"date"`
	NightlyTemperatureCelsius       interface{} `json:"nightlyTemperatureCelsius"`
	BaselineTemperatureCelsius      interface{} `json:"baselineTemperatureCelsius"`
	RelativeNightlyStddev30dCelsius interface{} `json:"relativeNightlyStddev30dCelsius"`
}

type WeightAPI struct {
	SampleTime  SampleTimeAPI `json:"sampleTime"`
	WeightGrams interface{}   `json:"weightGrams"`
}

type BodyFatAPI struct {
	SampleTime SampleTimeAPI `json:"sampleTime"`
	Percentage interface{}   `json:"percentage"`
}

type HydrationAmountAPI struct {
	Milliliters interface{} `json:"milliliters"`
}

type HydrationLogAPI struct {
	Interval       SessionTimeInterval `json:"interval"`
	AmountConsumed HydrationAmountAPI  `json:"amountConsumed"`
}

type NutrientQuantityAPI struct {
	Grams      interface{} `json:"grams"`
	Milligrams interface{} `json:"milligrams"`
}

type NutrientEntryAPI struct {
	Nutrient string              `json:"nutrient"`
	Quantity NutrientQuantityAPI `json:"quantity"`
}

type EnergyValueAPI struct {
	Kcal interface{} `json:"kcal"`
}

type MacroGramsAPI struct {
	Grams interface{} `json:"grams"`
}

type NutritionLogAPI struct {
	Interval          SessionTimeInterval `json:"interval"`
	FoodDisplayName   string              `json:"foodDisplayName"`
	MealType          string              `json:"mealType"`
	Energy            *EnergyValueAPI     `json:"energy,omitempty"`
	TotalCarbohydrate *MacroGramsAPI      `json:"totalCarbohydrate,omitempty"`
	TotalFat          *MacroGramsAPI      `json:"totalFat,omitempty"`
	Nutrients         []NutrientEntryAPI  `json:"nutrients,omitempty"`
}

type DataPoint struct {
	Name                             string                               `json:"name"`
	Sleep                            *Sleep                               `json:"sleep,omitempty"`
	DailyRestingHeartRate            *DailyRestingHeartRate               `json:"dailyRestingHeartRate,omitempty"`
	DailyHeartRateVariability        *DailyHeartVariable                  `json:"dailyHeartRateVariability,omitempty"`
	Steps                            *Steps                               `json:"steps,omitempty"`
	ActiveEnergyBurned               *ActiveEnergyBurned                  `json:"activeEnergyBurned,omitempty"`
	BasalEnergyBurned                *BasalEnergyBurnedAPI                `json:"basalEnergyBurned,omitempty"`
	ActiveMinutes                    *ActiveMinutes                       `json:"activeMinutes,omitempty"`
	Distance                         *DistanceAPI                         `json:"distance,omitempty"`
	Floors                           *FloorsAPI                           `json:"floors,omitempty"`
	TimeInHeartRateZone              *TimeInHeartRateZoneAPI              `json:"timeInHeartRateZone,omitempty"`
	DailyVO2Max                      *DailyVO2MaxAPI                      `json:"dailyVo2Max,omitempty"`
	VO2Max                           *VO2MaxAPI                           `json:"vo2Max,omitempty"`
	Exercise                         *ExerciseAPI                         `json:"exercise,omitempty"`
	DailyOxygenSaturation            *DailyOxygenSaturationAPI            `json:"dailyOxygenSaturation,omitempty"`
	DailyRespiratoryRate             *DailyRespiratoryRateAPI             `json:"dailyRespiratoryRate,omitempty"`
	DailySleepTemperatureDerivations *DailySleepTemperatureDerivationsAPI `json:"dailySleepTemperatureDerivations,omitempty"`
	Weight                           *WeightAPI                           `json:"weight,omitempty"`
	BodyFat                          *BodyFatAPI                          `json:"bodyFat,omitempty"`
	HydrationLog                     *HydrationLogAPI                     `json:"hydrationLog,omitempty"`
	NutritionLog                     *NutritionLogAPI                     `json:"nutritionLog,omitempty"`
}

type ListDataPointsResponse struct {
	DataPoints    []DataPoint `json:"dataPoints"`
	NextPageToken string      `json:"nextPageToken"`
}

type StatelessService struct {
	clientID     string
	clientSecret string
	redirectURI  string
	mu           sync.Mutex
	backoffUntil time.Time
	tokenMu      sync.Mutex
	latestTokens map[string]*oauth2.Token
}

func NewStatelessService(clientID, clientSecret, redirectURI string) *StatelessService {
	SetCookieEncryptionKey(clientSecret)
	return &StatelessService{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURI:  strings.TrimSpace(redirectURI),
		latestTokens: make(map[string]*oauth2.Token),
	}
}

func (s *StatelessService) GetCredentials() (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clientID, s.clientSecret, s.redirectURI
}

func (s *StatelessService) getWithRateLimit(client *http.Client, urlStr string) (*http.Response, error) {
	s.mu.Lock()
	backoff := s.backoffUntil
	s.mu.Unlock()

	if time.Now().Before(backoff) {
		waitTime := time.Until(backoff).Seconds()
		return nil, fmt.Errorf("RATE_LIMIT_BACKOFF: Google Health API rate limited. Please wait %.0f seconds", waitTime)
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		s.mu.Lock()
		retryAfterSec := 60
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if val, err := strconv.Atoi(ra); err == nil && val > 0 {
				retryAfterSec = val
			}
		}
		s.backoffUntil = time.Now().Add(time.Duration(retryAfterSec) * time.Second)
		s.mu.Unlock()
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("RATE_LIMIT_BACKOFF: Rate limited (429): %s. Backing off for %d seconds", string(body), retryAfterSec)
	}

	return resp, nil
}

// GetOAuthConfig returns a standardized oauth2.Config instance
func (s *StatelessService) GetOAuthConfig(redirectURI string) *oauth2.Config {
	cID, cSec, defaultRedirect := s.GetCredentials()
	if redirectURI == "" {
		redirectURI = defaultRedirect
	}
	return &oauth2.Config{
		ClientID:     cID,
		ClientSecret: cSec,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		RedirectURL: redirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/googlehealth.sleep.readonly",
			"https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly",
			"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly",
			"https://www.googleapis.com/auth/googlehealth.location.readonly",
			"https://www.googleapis.com/auth/googlehealth.nutrition.readonly",
			"https://www.googleapis.com/auth/googlehealth.profile.readonly",
			"https://www.googleapis.com/auth/googlehealth.irn.readonly",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}
}

// Helper to get OAuth Client and auto-refresh token in cookie if changed
func (s *StatelessService) GetOAuthClient(ctx context.Context, token *oauth2.Token, w http.ResponseWriter, r *http.Request) (*http.Client, error) {
	cID, cSec, _ := s.GetCredentials()
	if cID == "" || cSec == "" {
		return nil, fmt.Errorf("server OAuth client credentials are not configured")
	}

	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	effectiveToken := token
	if token.RefreshToken != "" {
		if cached, ok := s.latestTokens[token.RefreshToken]; ok && cached.Valid() {
			effectiveToken = cached
		}
	}

	config := s.GetOAuthConfig("")
	ts := config.TokenSource(ctx, effectiveToken)
	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh valid token: %w", err)
	}

	if token.RefreshToken != "" {
		s.latestTokens[token.RefreshToken] = newToken
	}
	if newToken.RefreshToken != "" {
		s.latestTokens[newToken.RefreshToken] = newToken
	}

	// If token refreshed, update browser cookie
	if newToken.AccessToken != token.AccessToken && w != nil {
		_ = SetSessionCookie(w, r, newToken)
	}

	return oauth2.NewClient(ctx, ts), nil
}

// Fetch User Profile from Google
func (s *StatelessService) FetchUserProfile(client *http.Client) (*UserProfile, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		resp, err = client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user profile, status %d", resp.StatusCode)
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// Fetch health metrics concurrently and return stateless response
func (s *StatelessService) FetchAllStats(client *http.Client, days int, metric, pageToken string) (*StatsResponse, error) {
	startTime := time.Now().UTC().AddDate(0, 0, -days)
	startTimeRFC := startTime.Format(time.RFC3339)
	startDateStr := startTime.Format("2006-01-02")

	resp := &StatsResponse{}
	var (
		errSleep error
		errRHR   error
		errHRV   error
		errAct   error
		errWork  error
		errVit   error
		errNut   error
		wg       sync.WaitGroup
	)

	fetchSlp := metric == "" || metric == "overview" || metric == "sleep"
	fetchR := metric == "" || metric == "overview" || metric == "heart" || metric == "rhr"
	fetchH := metric == "" || metric == "overview" || metric == "heart" || metric == "hrv"
	fetchHeartExtra := metric == "" || metric == "heart"
	fetchA := metric == "" || metric == "overview" || metric == "activity"
	fetchW := metric == "" || metric == "workouts"
	fetchV := metric == "" || metric == "vitals"
	fetchN := metric == "" || metric == "nutrition"

	if fetchSlp {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var token string
			if metric == "sleep" {
				token = pageToken
			}
			resp.SleepSessions, resp.SleepNextPageToken, errSleep = s.fetchSleep(client, startTimeRFC, token)
		}()
	}

	if fetchR {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var token string
			if metric == "rhr" || (metric == "heart" && pageToken != "") {
				token = pageToken
			}
			resp.RHRRecords, resp.RHRNextPageToken, errRHR = s.fetchRHR(client, startDateStr, token)
		}()
	}

	if fetchH {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var token string
			if metric == "hrv" {
				token = pageToken
			}
			resp.HRVRecords, resp.HRVNextPageToken, errHRV = s.fetchHRV(client, startDateStr, token)
		}()
	}

	if fetchHeartExtra {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp.HRZoneRecords, resp.VO2MaxRecords = s.fetchHRZonesAndVO2Max(client, startDateStr)
		}()
	}

	if fetchA {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var token string
			if metric == "activity" {
				token = pageToken
			}
			resp.ActivityRecords, resp.ActivityNextPageToken, errAct = s.fetchActivity(client, startTimeRFC, startDateStr, token)
		}()
	}

	if fetchW {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var token string
			if metric == "workouts" {
				token = pageToken
			}
			resp.ExerciseSessions, resp.ExerciseNextPageToken, errWork = s.fetchExercises(client, startTimeRFC, token)
		}()
	}

	if fetchV {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp.SpO2Records, resp.RespiratoryRecords, resp.SleepTempRecords, resp.BodyRecords, errVit = s.fetchVitalsAndBody(client, startDateStr)
		}()
	}

	if fetchN {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp.NutritionRecords, errNut = s.fetchNutrition(client, startDateStr)
		}()
	}

	wg.Wait()

	isScopeErr := func(err error) bool {
		if err == nil {
			return false
		}
		s := err.Error()
		return strings.Contains(s, "403") || strings.Contains(s, "insufficient") || strings.Contains(s, "PERMISSION_DENIED") || strings.Contains(s, "scopes")
	}

	addMissingScope := func(sc string) {
		for _, existing := range resp.MissingScopes {
			if existing == sc {
				return
			}
		}
		resp.MissingScopes = append(resp.MissingScopes, sc)
	}

	if errSleep != nil && fetchSlp {
		if strings.Contains(errSleep.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errSleep
		}
		if isScopeErr(errSleep) {
			addMissingScope("sleep")
		}
		fmt.Printf("[Stateless Fetch] Sleep error: %v\n", errSleep)
	} else if fetchSlp {
		fmt.Printf("[Stateless Fetch] Retrieved %d real sleep sessions from Google Health API\n", len(resp.SleepSessions))
	}

	if errRHR != nil && fetchR {
		if strings.Contains(errRHR.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errRHR
		}
		if isScopeErr(errRHR) {
			addMissingScope("heart")
		}
		fmt.Printf("[Stateless Fetch] RHR error: %v\n", errRHR)
	} else if fetchR {
		fmt.Printf("[Stateless Fetch] Retrieved %d real resting heart rate records from Google Health API\n", len(resp.RHRRecords))
	}

	if errHRV != nil && fetchH {
		if strings.Contains(errHRV.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errHRV
		}
		if isScopeErr(errHRV) {
			addMissingScope("heart")
		}
		fmt.Printf("[Stateless Fetch] HRV error: %v\n", errHRV)
	} else if fetchH {
		fmt.Printf("[Stateless Fetch] Retrieved %d real HRV records from Google Health API\n", len(resp.HRVRecords))
	}

	if errAct != nil && fetchA {
		if strings.Contains(errAct.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errAct
		}
		if isScopeErr(errAct) {
			addMissingScope("activity")
		}
		fmt.Printf("[Stateless Fetch] Activity error: %v\n", errAct)
	} else if fetchA {
		fmt.Printf("[Stateless Fetch] Retrieved %d real activity records from Google Health API\n", len(resp.ActivityRecords))
	}

	if errWork != nil && fetchW {
		if strings.Contains(errWork.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errWork
		}
		if isScopeErr(errWork) {
			addMissingScope("workouts")
		}
		fmt.Printf("[Stateless Fetch] Workouts error: %v\n", errWork)
	}

	if errVit != nil && fetchV {
		if strings.Contains(errVit.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errVit
		}
		if isScopeErr(errVit) {
			addMissingScope("vitals")
		}
		fmt.Printf("[Stateless Fetch] Vitals error: %v\n", errVit)
	}

	if errNut != nil && fetchN {
		if strings.Contains(errNut.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errNut
		}
		if isScopeErr(errNut) {
			addMissingScope("nutrition")
		}
		fmt.Printf("[Stateless Fetch] Nutrition error: %v\n", errNut)
	}

	if resp.SleepSessions == nil {
		resp.SleepSessions = make([]SleepSession, 0)
	}
	if resp.RHRRecords == nil {
		resp.RHRRecords = make([]RHRRecord, 0)
	}
	if resp.HRVRecords == nil {
		resp.HRVRecords = make([]HRVRecord, 0)
	}
	if resp.HRZoneRecords == nil {
		resp.HRZoneRecords = make([]HRZoneRecord, 0)
	}
	if resp.VO2MaxRecords == nil {
		resp.VO2MaxRecords = make([]VO2MaxRecord, 0)
	}
	if resp.ActivityRecords == nil {
		resp.ActivityRecords = make([]ActivityRecord, 0)
	}
	if resp.ExerciseSessions == nil {
		resp.ExerciseSessions = make([]ExerciseSession, 0)
	}
	if resp.SpO2Records == nil {
		resp.SpO2Records = make([]SpO2Record, 0)
	}
	if resp.RespiratoryRecords == nil {
		resp.RespiratoryRecords = make([]RespiratoryRateRecord, 0)
	}
	if resp.SleepTempRecords == nil {
		resp.SleepTempRecords = make([]SleepTempRecord, 0)
	}
	if resp.BodyRecords == nil {
		resp.BodyRecords = make([]BodyRecord, 0)
	}
	if resp.NutritionRecords == nil {
		resp.NutritionRecords = make([]NutritionRecord, 0)
	}

	sort.Slice(resp.SleepSessions, func(i, j int) bool {
		return resp.SleepSessions[i].StartTime > resp.SleepSessions[j].StartTime
	})
	sort.Slice(resp.RHRRecords, func(i, j int) bool {
		return resp.RHRRecords[i].Date > resp.RHRRecords[j].Date
	})
	sort.Slice(resp.HRVRecords, func(i, j int) bool {
		return resp.HRVRecords[i].Date > resp.HRVRecords[j].Date
	})
	sort.Slice(resp.HRZoneRecords, func(i, j int) bool {
		return resp.HRZoneRecords[i].Date > resp.HRZoneRecords[j].Date
	})
	sort.Slice(resp.VO2MaxRecords, func(i, j int) bool {
		return resp.VO2MaxRecords[i].Date > resp.VO2MaxRecords[j].Date
	})
	sort.Slice(resp.ActivityRecords, func(i, j int) bool {
		return resp.ActivityRecords[i].Date > resp.ActivityRecords[j].Date
	})
	sort.Slice(resp.ExerciseSessions, func(i, j int) bool {
		return resp.ExerciseSessions[i].StartTime > resp.ExerciseSessions[j].StartTime
	})
	sort.Slice(resp.SpO2Records, func(i, j int) bool {
		return resp.SpO2Records[i].Date > resp.SpO2Records[j].Date
	})
	sort.Slice(resp.RespiratoryRecords, func(i, j int) bool {
		return resp.RespiratoryRecords[i].Date > resp.RespiratoryRecords[j].Date
	})
	sort.Slice(resp.SleepTempRecords, func(i, j int) bool {
		return resp.SleepTempRecords[i].Date > resp.SleepTempRecords[j].Date
	})
	sort.Slice(resp.BodyRecords, func(i, j int) bool {
		return resp.BodyRecords[i].Date > resp.BodyRecords[j].Date
	})
	sort.Slice(resp.NutritionRecords, func(i, j int) bool {
		return resp.NutritionRecords[i].Date > resp.NutritionRecords[j].Date
	})

	return resp, nil
}

func (s *StatelessService) fetchSleep(client *http.Client, startStr, pageToken string) ([]SleepSession, string, error) {
	urlStr := "https://health.googleapis.com/v4/users/me/dataTypes/sleep/dataPoints"
	u, _ := url.Parse(urlStr)
	q := u.Query()
	q.Set("pageSize", "50000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()
	urlStr = u.String()

	var sessions []SleepSession

	resp, err := s.getWithRateLimit(client, urlStr)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp ListDataPointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, "", err
	}

	for _, dp := range listResp.DataPoints {
		if dp.Sleep == nil {
			continue
		}

		parts := strings.Split(dp.Name, "/")
		sessionID := parts[len(parts)-1]

		asleep := 0
		awake := 0
		duration := 0

		if dp.Sleep.Summary != nil {
			asleep, _ = strconv.Atoi(dp.Sleep.Summary.MinutesAsleep)
			awake, _ = strconv.Atoi(dp.Sleep.Summary.MinutesAwake)
			duration, _ = strconv.Atoi(dp.Sleep.Summary.MinutesInSleepPeriod)
		}

		if duration == 0 {
			st, err1 := time.Parse(time.RFC3339, dp.Sleep.Interval.StartTime)
			et, err2 := time.Parse(time.RFC3339, dp.Sleep.Interval.EndTime)
			if err1 == nil && err2 == nil {
				duration = int(et.Sub(st).Minutes())
			}
		}

		session := SleepSession{
			ID:              sessionID,
			StartTime:       dp.Sleep.Interval.StartTime,
			EndTime:         dp.Sleep.Interval.EndTime,
			DurationMinutes: duration,
			MinutesAsleep:   asleep,
			MinutesAwake:    awake,
			SleepType:       dp.Sleep.Type,
		}

		for _, stageAPI := range dp.Sleep.Stages {
			st, err1 := time.Parse(time.RFC3339, stageAPI.StartTime)
			et, err2 := time.Parse(time.RFC3339, stageAPI.EndTime)
			stageDuration := 0
			if err1 == nil && err2 == nil {
				stageDuration = int(et.Sub(st).Minutes())
			}

			session.Stages = append(session.Stages, SleepStage{
				StageType:       stageAPI.Type,
				StartTime:       stageAPI.StartTime,
				EndTime:         stageAPI.EndTime,
				DurationMinutes: stageDuration,
			})
		}

		sessions = append(sessions, session)
	}

	var filtered []SleepSession
	for _, s := range sessions {
		if pageToken != "" || s.StartTime >= startStr || s.EndTime >= startStr || s.StartTime == "" {
			filtered = append(filtered, s)
		}
	}

	return filtered, listResp.NextPageToken, nil
}

func (s *StatelessService) fetchRHR(client *http.Client, startDateStr, pageToken string) ([]RHRRecord, string, error) {
	urlStr := "https://health.googleapis.com/v4/users/me/dataTypes/daily-resting-heart-rate/dataPoints"
	u, _ := url.Parse(urlStr)
	q := u.Query()
	q.Set("pageSize", "50000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()
	urlStr = u.String()

	var records []RHRRecord

	resp, err := s.getWithRateLimit(client, urlStr)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp ListDataPointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, "", err
	}

	for _, dp := range listResp.DataPoints {
		if dp.DailyRestingHeartRate == nil {
			continue
		}

		dateStr := dp.DailyRestingHeartRate.Date.String()
		bpm, _ := strconv.Atoi(dp.DailyRestingHeartRate.BeatsPerMinute)

		records = append(records, RHRRecord{
			Date:           dateStr,
			BeatsPerMinute: bpm,
		})
	}

	var filtered []RHRRecord
	for _, r := range records {
		if pageToken != "" || r.Date >= startDateStr || r.Date == "" {
			filtered = append(filtered, r)
		}
	}

	return filtered, listResp.NextPageToken, nil
}

func (s *StatelessService) fetchHRV(client *http.Client, startDateStr, pageToken string) ([]HRVRecord, string, error) {
	urlStr := "https://health.googleapis.com/v4/users/me/dataTypes/daily-heart-rate-variability/dataPoints"
	u, _ := url.Parse(urlStr)
	q := u.Query()
	q.Set("pageSize", "50000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()
	urlStr = u.String()

	var records []HRVRecord

	resp, err := s.getWithRateLimit(client, urlStr)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp ListDataPointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, "", err
	}

	for _, dp := range listResp.DataPoints {
		if dp.DailyHeartRateVariability == nil {
			continue
		}

		hrv := dp.DailyHeartRateVariability
		dateStr := hrv.Date.String()
		avg := 0.0
		if hrv.AverageHeartRateVariabilityMilliseconds != nil {
			avg = *hrv.AverageHeartRateVariabilityMilliseconds
		}

		records = append(records, HRVRecord{
			Date:           dateStr,
			AvgHRVMs:       avg,
			Entropy:        hrv.Entropy,
			DeepSleepRMSSD: hrv.DeepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds,
		})
	}

	var filtered []HRVRecord
	for _, r := range records {
		if pageToken != "" || r.Date >= startDateStr || r.Date == "" {
			filtered = append(filtered, r)
		}
	}

	return filtered, listResp.NextPageToken, nil
}

func (s *StatelessService) fetchActivity(client *http.Client, startStr, startDateStr, pageToken string) ([]ActivityRecord, string, error) {
	stepsMap := make(map[string]int)
	caloriesMap := make(map[string]int)
	basalMap := make(map[string]int)
	minutesMap := make(map[string]int)
	distanceMap := make(map[string]float64)
	floorsMap := make(map[string]int)

	getDateStr := func(civ *CivilDateTime, startRaw string) string {
		if civ != nil {
			return civ.Date.String()
		}
		t, err := time.Parse(time.RFC3339, startRaw)
		if err == nil {
			return t.Format("2006-01-02")
		}
		return ""
	}

	buildURL := func(dataType string) string {
		u, _ := url.Parse("https://health.googleapis.com/v4/users/me/dataTypes/" + dataType + "/dataPoints")
		q := u.Query()
		q.Set("pageSize", "50000")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	// 1. Fetch Steps
	var nextStepsToken string
	resp, err := s.getWithRateLimit(client, buildURL("steps"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				nextStepsToken = listResp.NextPageToken
				for _, dp := range listResp.DataPoints {
					if dp.Steps != nil {
						d := getDateStr(dp.Steps.Interval.CivilStartTime, dp.Steps.Interval.StartTime)
						if d != "" {
							cnt, _ := strconv.Atoi(dp.Steps.Count)
							stepsMap[d] += cnt
						}
					}
				}
			}
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("[Stateless Fetch] Steps API error (%d): %s\n", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}

	// 2. Fetch Active Energy Burned
	resp, err = s.getWithRateLimit(client, buildURL("active-energy-burned"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.ActiveEnergyBurned != nil {
						d := getDateStr(dp.ActiveEnergyBurned.Interval.CivilStartTime, dp.ActiveEnergyBurned.Interval.StartTime)
						if d != "" {
							caloriesMap[d] += int(dp.ActiveEnergyBurned.Kcal)
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 3. Fetch Basal Energy Burned
	resp, err = s.getWithRateLimit(client, buildURL("basal-energy-burned"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.BasalEnergyBurned != nil {
						d := getDateStr(dp.BasalEnergyBurned.Interval.CivilStartTime, dp.BasalEnergyBurned.Interval.StartTime)
						if d != "" {
							basalMap[d] += parseAnyInt(dp.BasalEnergyBurned.Kcal)
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 4. Fetch Active Minutes
	resp, err = s.getWithRateLimit(client, buildURL("active-minutes"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.ActiveMinutes != nil {
						d := getDateStr(dp.ActiveMinutes.Interval.CivilStartTime, dp.ActiveMinutes.Interval.StartTime)
						if d != "" {
							subSum := 0
							for _, level := range dp.ActiveMinutes.ActiveMinutesByActivityLevel {
								val, _ := strconv.Atoi(level.ActiveMinutes)
								subSum += val
							}
							minutesMap[d] += subSum
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 5. Fetch Distance
	resp, err = s.getWithRateLimit(client, buildURL("distance"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.Distance != nil {
						d := getDateStr(dp.Distance.Interval.CivilStartTime, dp.Distance.Interval.StartTime)
						if d != "" {
							mm := parseAnyFloat(dp.Distance.DistanceMillimeters)
							if mm == 0 {
								mm = parseAnyFloat(dp.Distance.Millimeters)
							}
							distanceMap[d] += mm / 1000000.0
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 6. Fetch Floors
	resp, err = s.getWithRateLimit(client, buildURL("floors"))
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.Floors != nil {
						d := getDateStr(dp.Floors.Interval.CivilStartTime, dp.Floors.Interval.StartTime)
						if d != "" {
							cnt, _ := strconv.Atoi(dp.Floors.Count)
							for _, v := range dp.Floors.Values {
								vc, _ := strconv.Atoi(v.Count)
								cnt += vc
							}
							floorsMap[d] += cnt
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	allDates := make(map[string]bool)
	for d := range stepsMap {
		allDates[d] = true
	}
	for d := range caloriesMap {
		allDates[d] = true
	}
	for d := range basalMap {
		allDates[d] = true
	}
	for d := range minutesMap {
		allDates[d] = true
	}
	for d := range distanceMap {
		allDates[d] = true
	}
	for d := range floorsMap {
		allDates[d] = true
	}

	var records []ActivityRecord
	for d := range allDates {
		if d >= startDateStr {
			distKm := distanceMap[d]
			if distKm == 0 && stepsMap[d] > 0 {
				distKm = math.Round(float64(stepsMap[d])*0.000762*100) / 100
			} else {
				distKm = math.Round(distKm*100) / 100
			}
			records = append(records, ActivityRecord{
				Date:           d,
				Steps:          stepsMap[d],
				CaloriesBurned: caloriesMap[d],
				BasalCalories:  basalMap[d],
				ActiveMinutes:  minutesMap[d],
				DistanceKm:     distKm,
				Floors:         floorsMap[d],
			})
		}
	}

	if len(records) == 0 {
		nextStepsToken = ""
	}

	return records, nextStepsToken, nil
}

func (s *StatelessService) fetchHRZonesAndVO2Max(client *http.Client, startDateStr string) ([]HRZoneRecord, []VO2MaxRecord) {
	var zones []HRZoneRecord
	var vo2s []VO2MaxRecord

	// 1. Fetch Time in Heart Rate Zone
	zoneMap := make(map[string]*HRZoneRecord)
	resp, err := s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/time-in-heart-rate-zone/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.TimeInHeartRateZone == nil {
						continue
					}
					z := dp.TimeInHeartRateZone
					startRaw := z.Interval.StartTime
					if startRaw == "" {
						startRaw = z.StartTime
					}
					endRaw := z.Interval.EndTime
					if endRaw == "" {
						endRaw = z.EndTime
					}
					var d string
					if z.Interval.CivilStartTime != nil {
						d = z.Interval.CivilStartTime.Date.String()
					} else if t, err := time.Parse(time.RFC3339, startRaw); err == nil {
						d = t.Format("2006-01-02")
					}
					if d == "" || d < startDateStr {
						continue
					}
					durMin := 1
					st, err1 := time.Parse(time.RFC3339, startRaw)
					et, err2 := time.Parse(time.RFC3339, endRaw)
					if err1 == nil && err2 == nil {
						m := int(math.Round(et.Sub(st).Minutes()))
						if m > 0 {
							durMin = m
						}
					}
					rec, ok := zoneMap[d]
					if !ok {
						rec = &HRZoneRecord{Date: d}
						zoneMap[d] = rec
					}
					zt := strings.ToUpper(z.HeartRateZoneType)
					switch {
					case strings.Contains(zt, "PEAK"):
						rec.PeakMinutes += durMin
					case strings.Contains(zt, "VIGOROUS") || strings.Contains(zt, "CARDIO"):
						rec.VigorousMinutes += durMin
					case strings.Contains(zt, "MODERATE") || strings.Contains(zt, "FAT_BURN"):
						rec.ModerateMinutes += durMin
					default:
						rec.LightMinutes += durMin
					}
				}
			}
		}
		resp.Body.Close()
	}
	for _, r := range zoneMap {
		zones = append(zones, *r)
	}

	// 2. Fetch Daily VO2 Max (with fallback to vo2-max sample endpoint)
	vo2Map := make(map[string]float64)
	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/daily-vo2-max/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.DailyVO2Max != nil {
						d := dp.DailyVO2Max.Date.String()
						val := parseAnyFloat(dp.DailyVO2Max.VO2Max)
						if val == 0 {
							val = parseAnyFloat(dp.DailyVO2Max.VO2MaxValue)
						}
						if d >= startDateStr && val > 0 {
							vo2Map[d] = math.Round(val*10) / 10
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	if len(vo2Map) == 0 {
		resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/vo2-max/dataPoints?pageSize=50000")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var listResp ListDataPointsResponse
				if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
					for _, dp := range listResp.DataPoints {
						if dp.VO2Max != nil {
							var d string
							if dp.VO2Max.SampleTime.CivilTime != nil {
								d = dp.VO2Max.SampleTime.CivilTime.Date.String()
							} else if t, err := time.Parse(time.RFC3339, dp.VO2Max.SampleTime.PhysicalTime); err == nil {
								d = t.Format("2006-01-02")
							}
							val := parseAnyFloat(dp.VO2Max.VO2MaxValue)
							if val == 0 {
								val = parseAnyFloat(dp.VO2Max.VO2Max)
							}
							if d >= startDateStr && val > 0 {
								vo2Map[d] = math.Round(val*10) / 10
							}
						}
					}
				}
			}
			resp.Body.Close()
		}
	}

	for d, v := range vo2Map {
		vo2s = append(vo2s, VO2MaxRecord{Date: d, VO2Max: v})
	}

	return zones, vo2s
}

func parseDurationMinutes(durStr string) int {
	if durStr == "" {
		return 0
	}
	if d, err := time.ParseDuration(durStr); err == nil {
		return int(math.Round(d.Minutes()))
	}
	s := strings.TrimSuffix(durStr, "s")
	if sec, err := strconv.ParseFloat(s, 64); err == nil {
		return int(math.Round(sec / 60.0))
	}
	return 0
}

func parseAnyFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	}
	return 0
}

func parseAnyInt(v interface{}) int {
	return int(math.Round(parseAnyFloat(v)))
}

func (s *StatelessService) fetchExercises(client *http.Client, startStr, pageToken string) ([]ExerciseSession, string, error) {
	u, _ := url.Parse("https://health.googleapis.com/v4/users/me/dataTypes/exercise/dataPoints")
	q := u.Query()
	q.Set("pageSize", "50000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()

	resp, err := s.getWithRateLimit(client, u.String())
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp ListDataPointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		fmt.Printf("[Stateless Fetch] Exercise JSON decode error: %v\n", err)
		return nil, "", err
	}

	var sessions []ExerciseSession
	for _, dp := range listResp.DataPoints {
		if dp.Exercise == nil {
			continue
		}
		ex := dp.Exercise
		parts := strings.Split(dp.Name, "/")
		exID := parts[len(parts)-1]

		startRaw := ex.Interval.StartTime
		endRaw := ex.Interval.EndTime
		if startRaw == "" && ex.Interval.CivilStartTime != nil {
			startRaw = ex.Interval.CivilStartTime.Date.String() + "T00:00:00Z"
		}

		duration := 0
		st, err1 := time.Parse(time.RFC3339, startRaw)
		et, err2 := time.Parse(time.RFC3339, endRaw)
		if err1 == nil && err2 == nil {
			duration = int(math.Round(et.Sub(st).Minutes()))
		}

		cals := parseAnyInt(ex.CaloriesKcal)
		distMm := parseAnyFloat(ex.DistanceMillimeters)
		steps := parseAnyInt(ex.Steps)
		avgHr := parseAnyInt(ex.AverageHeartRateBeatsPerMinute)
		azm := 0
		var hz *HeartRateZoneDurationsAPI = ex.HeartRateZoneDurations

		if ex.MetricsSummary != nil {
			ms := ex.MetricsSummary
			if c := parseAnyInt(ms.CaloriesKcal); c > 0 {
				cals = c
			}
			if d := parseAnyFloat(ms.DistanceMillimeters); d > 0 {
				distMm = d
			}
			if s := parseAnyInt(ms.Steps); s > 0 {
				steps = s
			}
			if hr := parseAnyInt(ms.AverageHeartRateBeatsPerMinute); hr > 0 {
				avgHr = hr
			}
			azm = parseAnyInt(ms.ActiveZoneMinutes)
			if ms.HeartRateZoneDurations != nil {
				hz = ms.HeartRateZoneDurations
			}
		}

		lightM, modM, vigM, peakM := 0, 0, 0, 0
		if hz != nil {
			lightM = parseDurationMinutes(hz.LightTime)
			modM = parseDurationMinutes(hz.ModerateTime)
			vigM = parseDurationMinutes(hz.VigorousTime)
			peakM = parseDurationMinutes(hz.PeakTime)
		}

		exType := ex.ExerciseType
		if exType == "" {
			exType = "WORKOUT"
		}
		dispName := ex.DisplayName
		if dispName == "" {
			dispName = strings.Title(strings.ToLower(strings.ReplaceAll(exType, "_", " ")))
		}

		distKm := math.Round((distMm/1000000.0)*100) / 100
		upperType := strings.ToUpper(exType)
		hasGPS := distKm > 0 || strings.Contains(upperType, "RUN") || strings.Contains(upperType, "WALK") || strings.Contains(upperType, "BIK") || strings.Contains(upperType, "CYCL") || strings.Contains(upperType, "HIK")

		if pageToken != "" || startRaw >= startStr || endRaw >= startStr || startRaw == "" {
			sessions = append(sessions, ExerciseSession{
				ID:                exID,
				StartTime:         startRaw,
				EndTime:           endRaw,
				ExerciseType:      exType,
				DisplayName:       dispName,
				DurationMinutes:   duration,
				CaloriesKcal:      cals,
				DistanceKm:        distKm,
				Steps:             steps,
				AvgHeartRate:      avgHr,
				ActiveZoneMinutes: azm,
				LightZoneMins:     lightM,
				ModerateZoneMins:  modM,
				VigorousZoneMins:  vigM,
				PeakZoneMins:      peakM,
				HasGPS:            hasGPS,
			})
		}
	}

	fmt.Printf("[Stateless Fetch] Retrieved %d real exercise sessions from Google Health API\n", len(sessions))
	return sessions, listResp.NextPageToken, nil
}

type tcxTrackpointXML struct {
	Time     string `xml:"Time"`
	Position *struct {
		LatitudeDegrees  float64 `xml:"LatitudeDegrees"`
		LongitudeDegrees float64 `xml:"LongitudeDegrees"`
	} `xml:"Position"`
	AltitudeMeters float64 `xml:"AltitudeMeters"`
	DistanceMeters float64 `xml:"DistanceMeters"`
	HeartRateBpm   *struct {
		Value int `xml:"Value"`
	} `xml:"HeartRateBpm"`
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// FetchExerciseRoute downloads and parses the TCX GPS coordinate stream for an exercise session
func (s *StatelessService) FetchExerciseRoute(client *http.Client, exerciseID string) (*ExerciseRouteResponse, error) {
	urlStr := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/exercise/dataPoints/%s:exportExerciseTcx?alt=media", url.PathEscape(exerciseID))
	resp, err := s.getWithRateLimit(client, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TCX export error (%d): %s", resp.StatusCode, string(body))
	}

	decoder := xml.NewDecoder(resp.Body)
	var points []RoutePoint
	var firstTime time.Time
	var prevTime time.Time
	var cumDist float64

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Trackpoint" {
			continue
		}
		var tp tcxTrackpointXML
		if err := decoder.DecodeElement(&tp, &se); err != nil {
			continue
		}
		if tp.Position == nil || (tp.Position.LatitudeDegrees == 0 && tp.Position.LongitudeDegrees == 0) {
			continue
		}

		t, _ := time.Parse(time.RFC3339, tp.Time)
		if firstTime.IsZero() && !t.IsZero() {
			firstTime = t
		}
		elapsed := 0
		if !firstTime.IsZero() && !t.IsZero() {
			elapsed = int(t.Sub(firstTime).Seconds())
		}

		dist := tp.DistanceMeters
		if len(points) > 0 {
			prev := points[len(points)-1]
			segDist := haversineMeters(prev.Lat, prev.Lng, tp.Position.LatitudeDegrees, tp.Position.LongitudeDegrees)
			if dist == 0 {
				cumDist += segDist
				dist = cumDist
			} else {
				cumDist = dist
			}
		}

		pace := 0.0
		if len(points) > 0 && !prevTime.IsZero() && !t.IsZero() {
			dtSec := t.Sub(prevTime).Seconds()
			ddMeters := dist - points[len(points)-1].DistanceMeters
			if dtSec > 0 && ddMeters > 0.5 {
				pace = (dtSec / 60.0) / (ddMeters / 1000.0)
				if pace > 20 {
					pace = 20
				}
			}
		}
		if !t.IsZero() {
			prevTime = t
		}

		hr := 0
		if tp.HeartRateBpm != nil {
			hr = tp.HeartRateBpm.Value
		}

		points = append(points, RoutePoint{
			Timestamp:      tp.Time,
			ElapsedSeconds: elapsed,
			Lat:            tp.Position.LatitudeDegrees,
			Lng:            tp.Position.LongitudeDegrees,
			AltitudeMeters: math.Round(tp.AltitudeMeters*10) / 10,
			DistanceMeters: math.Round(dist*10) / 10,
			HeartRateBpm:   hr,
			PaceMinPerKm:   math.Round(pace*100) / 100,
		})
	}

	return &ExerciseRouteResponse{
		ExerciseID: exerciseID,
		HasRoute:   len(points) > 0,
		Points:     points,
	}, nil
}

func (s *StatelessService) fetchVitalsAndBody(client *http.Client, startDateStr string) ([]SpO2Record, []RespiratoryRateRecord, []SleepTempRecord, []BodyRecord, error) {
	var (
		spo2s []SpO2Record
		resps []RespiratoryRateRecord
		temps []SleepTempRecord
		bodys []BodyRecord
	)

	// 1. Daily Oxygen Saturation (SpO2)
	resp, err := s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/daily-oxygen-saturation/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.DailyOxygenSaturation != nil {
						o := dp.DailyOxygenSaturation
						d := o.Date.String()
						avgPct := parseAnyFloat(o.AveragePercentage)
						minPct := parseAnyFloat(o.LowerBoundPercentage)
						maxPct := parseAnyFloat(o.UpperBoundPercentage)
						if d >= startDateStr && avgPct > 0 {
							spo2s = append(spo2s, SpO2Record{
								Date:       d,
								AvgPercent: math.Round(avgPct*10) / 10,
								MinPercent: math.Round(minPct*10) / 10,
								MaxPercent: math.Round(maxPct*10) / 10,
							})
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 2. Daily Respiratory Rate
	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/daily-respiratory-rate/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.DailyRespiratoryRate != nil {
						rr := dp.DailyRespiratoryRate
						d := rr.Date.String()
						val := parseAnyFloat(rr.RateBpm)
						if val == 0 {
							val = parseAnyFloat(rr.BreathsPerMinute)
						}
						if d >= startDateStr && val > 0 {
							resps = append(resps, RespiratoryRateRecord{
								Date:             d,
								BreathsPerMinute: math.Round(val*10) / 10,
							})
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 3. Daily Sleep Temperature Derivations
	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/daily-sleep-temperature-derivations/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.DailySleepTemperatureDerivations != nil {
						st := dp.DailySleepTemperatureDerivations
						d := st.Date.String()
						nightlyC := parseAnyFloat(st.NightlyTemperatureCelsius)
						baseC := parseAnyFloat(st.BaselineTemperatureCelsius)
						if d >= startDateStr && nightlyC > 0 {
							base := nightlyC
							if baseC > 0 {
								base = baseC
							}
							delta := math.Round((nightlyC-base)*100) / 100
							temps = append(temps, SleepTempRecord{
								Date:         d,
								NightlyTempC: math.Round(nightlyC*100) / 100,
								BaselineC:    math.Round(base*100) / 100,
								DeltaC:       delta,
							})
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 4. Weight & Body Fat
	weightMap := make(map[string]float64)
	fatMap := make(map[string]float64)

	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/weight/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.Weight != nil {
						wg := parseAnyFloat(dp.Weight.WeightGrams)
						if wg > 0 {
							var d string
							if dp.Weight.SampleTime.CivilTime != nil {
								d = dp.Weight.SampleTime.CivilTime.Date.String()
							} else if t, err := time.Parse(time.RFC3339, dp.Weight.SampleTime.PhysicalTime); err == nil {
								d = t.Format("2006-01-02")
							}
							if d >= startDateStr {
								weightMap[d] = math.Round((wg/1000.0)*10) / 10
							}
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/body-fat/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.BodyFat != nil {
						pct := parseAnyFloat(dp.BodyFat.Percentage)
						if pct > 0 {
							var d string
							if dp.BodyFat.SampleTime.CivilTime != nil {
								d = dp.BodyFat.SampleTime.CivilTime.Date.String()
							} else if t, err := time.Parse(time.RFC3339, dp.BodyFat.SampleTime.PhysicalTime); err == nil {
								d = t.Format("2006-01-02")
							}
							if d >= startDateStr {
								fatMap[d] = math.Round(pct*10) / 10
							}
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	bodyDates := make(map[string]bool)
	for d := range weightMap {
		bodyDates[d] = true
	}
	for d := range fatMap {
		bodyDates[d] = true
	}
	for d := range bodyDates {
		rec := BodyRecord{Date: d}
		if w, ok := weightMap[d]; ok {
			wCopy := w
			rec.WeightKg = &wCopy
		}
		if f, ok := fatMap[d]; ok {
			fCopy := f
			rec.BodyFatPct = &fCopy
		}
		bodys = append(bodys, rec)
	}

	return spo2s, resps, temps, bodys, nil
}

func (s *StatelessService) fetchNutrition(client *http.Client, startDateStr string) ([]NutritionRecord, error) {
	nutMap := make(map[string]*NutritionRecord)
	getRec := func(d string) *NutritionRecord {
		r, ok := nutMap[d]
		if !ok {
			r = &NutritionRecord{Date: d}
			nutMap[d] = r
		}
		return r
	}

	// 1. Hydration Log
	resp, err := s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/hydration-log/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.HydrationLog != nil {
						var d string
						if dp.HydrationLog.Interval.CivilStartTime != nil {
							d = dp.HydrationLog.Interval.CivilStartTime.Date.String()
						} else if t, err := time.Parse(time.RFC3339, dp.HydrationLog.Interval.StartTime); err == nil {
							d = t.Format("2006-01-02")
						}
						if d >= startDateStr {
							getRec(d).HydrationMl += parseAnyInt(dp.HydrationLog.AmountConsumed.Milliliters)
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	// 2. Nutrition Log
	resp, err = s.getWithRateLimit(client, "https://health.googleapis.com/v4/users/me/dataTypes/nutrition-log/dataPoints?pageSize=50000")
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			var listResp ListDataPointsResponse
			if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
				for _, dp := range listResp.DataPoints {
					if dp.NutritionLog != nil {
						nl := dp.NutritionLog
						var d string
						if nl.Interval.CivilStartTime != nil {
							d = nl.Interval.CivilStartTime.Date.String()
						} else if t, err := time.Parse(time.RFC3339, nl.Interval.StartTime); err == nil {
							d = t.Format("2006-01-02")
						}
						if d >= startDateStr {
							rec := getRec(d)
							rec.MealsLogged++
							if nl.Energy != nil {
								rec.CaloriesIn += parseAnyInt(nl.Energy.Kcal)
							}
							if nl.TotalCarbohydrate != nil {
								rec.CarbsG = math.Round((rec.CarbsG+parseAnyFloat(nl.TotalCarbohydrate.Grams))*10) / 10
							}
							if nl.TotalFat != nil {
								rec.FatG = math.Round((rec.FatG+parseAnyFloat(nl.TotalFat.Grams))*10) / 10
							}
							for _, nut := range nl.Nutrients {
								if strings.Contains(strings.ToUpper(nut.Nutrient), "PROTEIN") {
									rec.ProteinG = math.Round((rec.ProteinG+parseAnyFloat(nut.Quantity.Grams))*10) / 10
								}
							}
						}
					}
				}
			}
		}
		resp.Body.Close()
	}

	var out []NutritionRecord
	for _, r := range nutMap {
		out = append(out, *r)
	}
	return out, nil
}

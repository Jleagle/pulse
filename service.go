package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type DataPoint struct {
	Name                      string                 `json:"name"`
	Sleep                     *Sleep                 `json:"sleep,omitempty"`
	DailyRestingHeartRate     *DailyRestingHeartRate `json:"dailyRestingHeartRate,omitempty"`
	DailyHeartRateVariability *DailyHeartVariable    `json:"dailyHeartRateVariability,omitempty"`
	Steps                     *Steps                 `json:"steps,omitempty"`
	ActiveEnergyBurned        *ActiveEnergyBurned    `json:"activeEnergyBurned,omitempty"`
	ActiveMinutes             *ActiveMinutes         `json:"activeMinutes,omitempty"`
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
}

func NewStatelessService(clientID, clientSecret, redirectURI string) *StatelessService {
	return &StatelessService{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
	}
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

// Helper to get OAuth Client and auto-refresh token in cookie if changed
func (s *StatelessService) GetOAuthClient(ctx context.Context, token *oauth2.Token, w http.ResponseWriter) (*http.Client, error) {
	if s.clientID == "" || s.clientSecret == "" {
		return nil, fmt.Errorf("server OAuth client credentials are not configured")
	}

	config := &oauth2.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		Scopes: []string{
			"https://www.googleapis.com/auth/googlehealth.sleep.readonly",
			"https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly",
			"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly",
			"https://www.googleapis.com/auth/googlehealth.profile.readonly",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}

	ts := config.TokenSource(ctx, token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh valid token: %w", err)
	}

	// If token refreshed, update browser cookie
	if newToken.AccessToken != token.AccessToken && w != nil {
		_ = SetSessionCookie(w, newToken)
	}

	return oauth2.NewClient(ctx, ts), nil
}

// Fetch User Profile from Google
func (s *StatelessService) FetchUserProfile(client *http.Client) (*UserProfile, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
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
		wg       sync.WaitGroup
	)

	fetchSlp := metric == "" || metric == "overview" || metric == "sleep"
	fetchR := metric == "" || metric == "overview" || metric == "heart" || metric == "rhr"
	fetchH := metric == "" || metric == "overview" || metric == "heart" || metric == "hrv"
	fetchA := metric == "" || metric == "overview" || metric == "activity"

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

	wg.Wait()

	if errSleep != nil && fetchSlp {
		if strings.Contains(errSleep.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errSleep
		}
		fmt.Printf("[Stateless Fetch] Sleep error: %v\n", errSleep)
	} else if fetchSlp {
		fmt.Printf("[Stateless Fetch] Retrieved %d real sleep sessions from Google Health API\n", len(resp.SleepSessions))
	}

	if errRHR != nil && fetchR {
		if strings.Contains(errRHR.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errRHR
		}
		fmt.Printf("[Stateless Fetch] RHR error: %v\n", errRHR)
	} else if fetchR {
		fmt.Printf("[Stateless Fetch] Retrieved %d real resting heart rate records from Google Health API\n", len(resp.RHRRecords))
	}

	if errHRV != nil && fetchH {
		if strings.Contains(errHRV.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errHRV
		}
		fmt.Printf("[Stateless Fetch] HRV error: %v\n", errHRV)
	} else if fetchH {
		fmt.Printf("[Stateless Fetch] Retrieved %d real HRV records from Google Health API\n", len(resp.HRVRecords))
	}

	if errAct != nil && fetchA {
		if strings.Contains(errAct.Error(), "RATE_LIMIT_BACKOFF") {
			return nil, errAct
		}
		fmt.Printf("[Stateless Fetch] Activity error: %v\n", errAct)
	} else if fetchA {
		fmt.Printf("[Stateless Fetch] Retrieved %d real activity records from Google Health API\n", len(resp.ActivityRecords))
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
	if resp.ActivityRecords == nil {
		resp.ActivityRecords = make([]ActivityRecord, 0)
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
	sort.Slice(resp.ActivityRecords, func(i, j int) bool {
		return resp.ActivityRecords[i].Date > resp.ActivityRecords[j].Date
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
	minutesMap := make(map[string]int)

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

	// 1. Fetch Steps
	stepsURL := "https://health.googleapis.com/v4/users/me/dataTypes/steps/dataPoints"
	u, _ := url.Parse(stepsURL)
	q := u.Query()
	q.Set("pageSize", "50000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()
	stepsURL = u.String()

	var nextStepsToken string
	resp, err := s.getWithRateLimit(client, stepsURL)
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
	energyURL := "https://health.googleapis.com/v4/users/me/dataTypes/active-energy-burned/dataPoints"
	u2, _ := url.Parse(energyURL)
	q2 := u2.Query()
	q2.Set("pageSize", "50000")
	if pageToken != "" {
		q2.Set("pageToken", pageToken)
	}
	u2.RawQuery = q2.Encode()
	energyURL = u2.String()

	resp, err = s.getWithRateLimit(client, energyURL)
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
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("[Stateless Fetch] Energy API error (%d): %s\n", resp.StatusCode, string(body))
		}
		resp.Body.Close()
	}

	// 3. Fetch Active Minutes
	minutesURL := "https://health.googleapis.com/v4/users/me/dataTypes/active-minutes/dataPoints"
	u3, _ := url.Parse(minutesURL)
	q3 := u3.Query()
	q3.Set("pageSize", "50000")
	if pageToken != "" {
		q3.Set("pageToken", pageToken)
	}
	u3.RawQuery = q3.Encode()
	minutesURL = u3.String()

	resp, err = s.getWithRateLimit(client, minutesURL)
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
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("[Stateless Fetch] Active Minutes API error (%d): %s\n", resp.StatusCode, string(body))
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
	for d := range minutesMap {
		allDates[d] = true
	}

	var records []ActivityRecord
	for d := range allDates {
		if pageToken != "" || d >= startDateStr {
			records = append(records, ActivityRecord{
				Date:           d,
				Steps:          stepsMap[d],
				CaloriesBurned: caloriesMap[d],
				ActiveMinutes:  minutesMap[d],
			})
		}
	}

	return records, nextStepsToken, nil
}

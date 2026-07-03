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
}

func NewStatelessService(clientID, clientSecret, redirectURI string) *StatelessService {
	return &StatelessService{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
	}
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

// Fetch all health metrics concurrently and return stateless response
func (s *StatelessService) FetchAllStats(client *http.Client, days int) (*StatsResponse, error) {
	startTime := time.Now().AddDate(0, 0, -days)
	startTimeRFC := startTime.Format(time.RFC3339)
	startDateStr := startTime.Format("2006-01-02")

	var (
		sleepSessions []SleepSession
		rhrRecords    []RHRRecord
		hrvRecords    []HRVRecord
		activityRecs  []ActivityRecord
		errSleep      error
		errRHR        error
		errHRV        error
		errAct        error
		wg            sync.WaitGroup
	)

	wg.Add(4)

	// Concurrent fetch: Sleep
	go func() {
		defer wg.Done()
		sleepSessions, errSleep = s.fetchSleep(client, startTimeRFC)
	}()

	// Concurrent fetch: RHR
	go func() {
		defer wg.Done()
		rhrRecords, errRHR = s.fetchRHR(client, startDateStr)
	}()

	// Concurrent fetch: HRV
	go func() {
		defer wg.Done()
		hrvRecords, errHRV = s.fetchHRV(client, startDateStr)
	}()

	// Concurrent fetch: Activity
	go func() {
		defer wg.Done()
		activityRecs, errAct = s.fetchActivity(client, startTimeRFC, startDateStr)
	}()

	wg.Wait()

	// Check if any critical API errors occurred
	if errSleep != nil {
		fmt.Printf("[Stateless Fetch] Sleep error: %v\n", errSleep)
	}
	if errRHR != nil {
		fmt.Printf("[Stateless Fetch] RHR error: %v\n", errRHR)
	}
	if errHRV != nil {
		fmt.Printf("[Stateless Fetch] HRV error: %v\n", errHRV)
	}
	if errAct != nil {
		fmt.Printf("[Stateless Fetch] Activity error: %v\n", errAct)
	}

	// Sort results latest first (descending by date/time)
	sort.Slice(sleepSessions, func(i, j int) bool {
		return sleepSessions[i].StartTime > sleepSessions[j].StartTime
	})
	sort.Slice(rhrRecords, func(i, j int) bool {
		return rhrRecords[i].Date > rhrRecords[j].Date
	})
	sort.Slice(hrvRecords, func(i, j int) bool {
		return hrvRecords[i].Date > hrvRecords[j].Date
	})
	sort.Slice(activityRecs, func(i, j int) bool {
		return activityRecs[i].Date > activityRecs[j].Date
	})

	return &StatsResponse{
		SleepSessions:   sleepSessions,
		RHRRecords:      rhrRecords,
		HRVRecords:      hrvRecords,
		ActivityRecords: activityRecs,
	}, nil
}

func (s *StatelessService) fetchSleep(client *http.Client, startStr string) ([]SleepSession, error) {
	filter := fmt.Sprintf("sleep.interval.end_time >= %q", startStr)
	urlStr := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/sleep/dataPoints?filter=%s", url.QueryEscape(filter))

	var sessions []SleepSession

	for {
		resp, err := client.Get(urlStr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var listResp ListDataPointsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			return nil, err
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

		if listResp.NextPageToken == "" {
			break
		}
		u, _ := url.Parse(urlStr)
		q := u.Query()
		q.Set("pageToken", listResp.NextPageToken)
		u.RawQuery = q.Encode()
		urlStr = u.String()
	}

	return sessions, nil
}

func (s *StatelessService) fetchRHR(client *http.Client, startDateStr string) ([]RHRRecord, error) {
	filter := fmt.Sprintf("daily_resting_heart_rate.date >= %q", startDateStr)
	urlStr := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/daily-resting-heart-rate/dataPoints?filter=%s", url.QueryEscape(filter))

	var records []RHRRecord

	for {
		resp, err := client.Get(urlStr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var listResp ListDataPointsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			return nil, err
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

		if listResp.NextPageToken == "" {
			break
		}
		u, _ := url.Parse(urlStr)
		q := u.Query()
		q.Set("pageToken", listResp.NextPageToken)
		u.RawQuery = q.Encode()
		urlStr = u.String()
	}

	return records, nil
}

func (s *StatelessService) fetchHRV(client *http.Client, startDateStr string) ([]HRVRecord, error) {
	filter := fmt.Sprintf("daily_heart_rate_variability.date >= %q", startDateStr)
	urlStr := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/daily-heart-rate-variability/dataPoints?filter=%s", url.QueryEscape(filter))

	var records []HRVRecord

	for {
		resp, err := client.Get(urlStr)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}

		var listResp ListDataPointsResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			return nil, err
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

		if listResp.NextPageToken == "" {
			break
		}
		u, _ := url.Parse(urlStr)
		q := u.Query()
		q.Set("pageToken", listResp.NextPageToken)
		u.RawQuery = q.Encode()
		urlStr = u.String()
	}

	return records, nil
}

func (s *StatelessService) fetchActivity(client *http.Client, startStr, startDateStr string) ([]ActivityRecord, error) {
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
	stepsFilter := fmt.Sprintf("steps.interval.start_time >= %q", startStr)
	stepsURL := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/steps/dataPoints?filter=%s", url.QueryEscape(stepsFilter))
	for {
		resp, err := client.Get(stepsURL)
		if err != nil {
			break
		}
		var listResp ListDataPointsResponse
		if json.NewDecoder(resp.Body).Decode(&listResp) == nil {
			for _, dp := range listResp.DataPoints {
				if dp.Steps != nil {
					d := getDateStr(dp.Steps.Interval.CivilStartTime, dp.Steps.Interval.StartTime)
					if d != "" {
						cnt, _ := strconv.Atoi(dp.Steps.Count)
						stepsMap[d] += cnt
					}
				}
			}
			resp.Body.Close()
			if listResp.NextPageToken == "" {
				break
			}
			u, _ := url.Parse(stepsURL)
			q := u.Query()
			q.Set("pageToken", listResp.NextPageToken)
			u.RawQuery = q.Encode()
			stepsURL = u.String()
		} else {
			resp.Body.Close()
			break
		}
	}

	// 2. Fetch Active Energy Burned
	energyFilter := fmt.Sprintf("active_energy_burned.interval.start_time >= %q", startStr)
	energyURL := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/active-energy-burned/dataPoints?filter=%s", url.QueryEscape(energyFilter))
	for {
		resp, err := client.Get(energyURL)
		if err != nil {
			break
		}
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
			resp.Body.Close()
			if listResp.NextPageToken == "" {
				break
			}
			u, _ := url.Parse(energyURL)
			q := u.Query()
			q.Set("pageToken", listResp.NextPageToken)
			u.RawQuery = q.Encode()
			energyURL = u.String()
		} else {
			resp.Body.Close()
			break
		}
	}

	// 3. Fetch Active Minutes
	minutesFilter := fmt.Sprintf("active_minutes.interval.start_time >= %q", startStr)
	minutesURL := fmt.Sprintf("https://health.googleapis.com/v4/users/me/dataTypes/active-minutes/dataPoints?filter=%s", url.QueryEscape(minutesFilter))
	for {
		resp, err := client.Get(minutesURL)
		if err != nil {
			break
		}
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
			resp.Body.Close()
			if listResp.NextPageToken == "" {
				break
			}
			u, _ := url.Parse(minutesURL)
			q := u.Query()
			q.Set("pageToken", listResp.NextPageToken)
			u.RawQuery = q.Encode()
			minutesURL = u.String()
		} else {
			resp.Body.Close()
			break
		}
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
		if d >= startDateStr {
			records = append(records, ActivityRecord{
				Date:           d,
				Steps:          stepsMap[d],
				CaloriesBurned: caloriesMap[d],
				ActiveMinutes:  minutesMap[d],
			})
		}
	}

	return records, nil
}

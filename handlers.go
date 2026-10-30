package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
)

//go:embed web/*
var webAssets embed.FS

type Server struct {
	service     *StatelessService
	redirectURI string
	password    string
}

func NewServer(service *StatelessService, redirectURI string) *Server {
	return &Server{
		service:     service,
		redirectURI: redirectURI,
	}
}

func (s *Server) SetPassword(password string) {
	s.password = strings.TrimSpace(password)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/auth-url", s.handleAuthURL)
	mux.HandleFunc("GET /oauth/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/logout", s.handleLogout)
	mux.HandleFunc("POST /api/cache/clear", s.handleClearCache)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/exercise-route", s.handleExerciseRoute)

	// Frontend static assets
	subFS, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err)
	}

	// SPA tab routing so page refreshes and links work cleanly
	for _, tab := range []string{"overview", "sleep", "heart", "activity", "workouts", "vitals", "nutrition", "settings"} {
		tabName := tab
		mux.HandleFunc("GET /"+tabName, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, subFS, "index.html")
		})
	}

	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
		}
		fileServer.ServeHTTP(w, r)
	}))

	if s.password != "" {
		return s.basicAuthMiddleware(mux)
	}
	return mux
}

func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	expected := []byte(s.password)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), expected) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Pulse Self-Hosted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) effectiveRedirectURI(r *http.Request) string {
	_, _, configuredRedirect := s.service.GetCredentials()
	if configuredRedirect != "" && configuredRedirect != "http://localhost:8080/oauth/callback" {
		return configuredRedirect
	}
	if r == nil {
		if configuredRedirect != "" {
			return configuredRedirect
		}
		return "http://localhost:8080/oauth/callback"
	}
	scheme := "http"
	if isSecureRequest(r) {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.Index(host, ","); idx != -1 {
		host = strings.TrimSpace(host[:idx])
	}
	if host == "" {
		if s.redirectURI != "" {
			return s.redirectURI
		}
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s/oauth/callback", scheme, host)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	cID, cSec, _ := s.service.GetCredentials()
	clientConfigured := cID != "" && cSec != ""
	oauthConnected := false
	userEmail := ""
	userName := ""
	userPic := ""

	token, err := GetSessionCookie(r)
	if err == nil && token != nil {
		client, err := s.service.GetOAuthClient(r.Context(), token, w, r)
		if err == nil {
			oauthConnected = true
			profile, err := s.service.FetchUserProfile(client)
			if err == nil && profile != nil {
				userEmail = profile.Email
				userName = profile.Name
				userPic = profile.Picture
			} else {
				log.Printf("[WARNING] FetchUserProfile failed (userinfo API might be disabled), using default user name: %v", err)
				userName = "Connected User"
			}
		} else {
			log.Printf("[WARNING] GetOAuthClient failed in handleStatus: %v", err)
			ClearSessionCookie(w, r)
		}
	} else if err != nil && !errors.Is(err, http.ErrNoCookie) && err.Error() != "empty session cookie" {
		log.Printf("[DEBUG] GetSessionCookie error in handleStatus: %v", err)
	}

	if userName == "" && userEmail != "" {
		parts := strings.Split(userEmail, "@")
		userName = parts[0]
	}

	resp := StatusResponse{
		ClientConfigured: clientConfigured,
		OAuthConnected:   oauthConnected,
		RedirectURI:      s.effectiveRedirectURI(r),
		UserEmail:        userEmail,
		UserName:         userName,
		UserPicture:      userPic,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAuthURL(w http.ResponseWriter, r *http.Request) {
	cID, cSec, _ := s.service.GetCredentials()
	if cID == "" || cSec == "" {
		http.Error(w, "OAuth Client ID or Secret not configured on server", http.StatusInternalServerError)
		return
	}

	config := s.service.GetOAuthConfig(s.effectiveRedirectURI(r))
	reqScope := r.URL.Query().Get("scope")
	if reqScope != "" {
		scopes := []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		}
		switch reqScope {
		case "sleep":
			scopes = append(scopes, "https://www.googleapis.com/auth/googlehealth.sleep.readonly")
		case "heart", "vitals":
			scopes = append(scopes, "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly")
		case "activity":
			scopes = append(scopes, "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly")
		case "workouts", "location":
			scopes = append(scopes,
				"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly",
				"https://www.googleapis.com/auth/googlehealth.location.readonly",
			)
		case "nutrition":
			scopes = append(scopes, "https://www.googleapis.com/auth/googlehealth.nutrition.readonly")
		default:
			scopes = append(scopes, reqScope)
		}
		config.Scopes = scopes
	}

	state, err := SetOAuthStateCookie(w, r)
	if err != nil {
		http.Error(w, "Failed to generate OAuth state token", http.StatusInternalServerError)
		return
	}

	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !ValidateOAuthStateCookie(w, r) {
		http.Redirect(w, r, "/?error=invalid_oauth_state", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error=no_code", http.StatusFound)
		return
	}

	cID, cSec, _ := s.service.GetCredentials()
	if cID == "" || cSec == "" {
		http.Redirect(w, r, "/?error=server_not_configured", http.StatusFound)
		return
	}

	config := s.service.GetOAuthConfig(s.effectiveRedirectURI(r))
	token, err := config.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("[ERROR] OAuth token exchange failed: %v", err)
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}

	if err := SetSessionCookie(w, r, token); err != nil {
		log.Printf("[ERROR] Failed to set session cookie: %v", err)
		http.Redirect(w, r, "/?error=cookie_error", http.StatusFound)
		return
	}
	s.service.ClearUserCache(token)

	log.Printf("✅ OAuth Exchange successful and session cookie set!")
	http.Redirect(w, r, "/?connected=true", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token, err := GetSessionCookie(r); err == nil && token != nil {
		s.service.ClearUserCache(token)
	}
	ClearSessionCookie(w, r)
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	if token, err := GetSessionCookie(r); err == nil && token != nil {
		s.service.ClearUserCache(token)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	token, err := GetSessionCookie(r)
	if err != nil || token == nil {
		http.Error(w, "Unauthorized: Not connected to Google Health", http.StatusUnauthorized)
		return
	}

	if r.URL.Query().Get("refresh") == "true" {
		s.service.ClearUserCache(token)
	}

	client, err := s.service.GetOAuthClient(r.Context(), token, w, r)
	if err != nil {
		ClearSessionCookie(w, r)
		http.Error(w, "Unauthorized: Invalid or expired session", http.StatusUnauthorized)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	days := 3650 // Default to 3650 days (10 years) of data for maximum allowed history
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 3650 {
			days = l
		}
	}

	metric := r.URL.Query().Get("metric")
	pageToken := r.URL.Query().Get("pageToken")

	stats, err := s.service.FetchAllStats(client, days, metric, pageToken)
	if err != nil {
		if strings.Contains(err.Error(), "RATE_LIMIT_BACKOFF") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "rate_limited",
				"message":     "Google Health API rate limit reached. Pausing requests...",
				"retry_after": 60,
			})
			return
		}
		http.Error(w, "Failed to fetch health stats from Google API: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleExerciseRoute(w http.ResponseWriter, r *http.Request) {
	exerciseID := r.URL.Query().Get("id")
	if exerciseID == "" {
		http.Error(w, "Missing exercise id", http.StatusBadRequest)
		return
	}

	token, err := GetSessionCookie(r)
	if err != nil || token == nil {
		http.Error(w, "Unauthorized: Not connected to Google Health", http.StatusUnauthorized)
		return
	}

	client, err := s.service.GetOAuthClient(r.Context(), token, w, r)
	if err != nil {
		ClearSessionCookie(w, r)
		http.Error(w, "Unauthorized: Invalid or expired session", http.StatusUnauthorized)
		return
	}

	routeResp, err := s.service.FetchExerciseRoute(client, exerciseID)
	if err != nil {
		log.Printf("[WARNING] FetchExerciseRoute failed for %s: %v", exerciseID, err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&ExerciseRouteResponse{
			ExerciseID: exerciseID,
			HasRoute:   false,
			Points:     []RoutePoint{},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routeResp)
}

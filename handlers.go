package main

import (
	"embed"
	"encoding/json"
	"errors"
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
}

func NewServer(service *StatelessService, redirectURI string) *Server {
	return &Server{
		service:     service,
		redirectURI: redirectURI,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/auth-url", s.handleAuthURL)
	mux.HandleFunc("GET /oauth/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/stats", s.handleStats)

	// Frontend static assets
	subFS, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err)
	}

	// SPA tab routing so page refreshes and links work cleanly
	for _, tab := range []string{"overview", "sleep", "heart", "activity", "settings", "privacy-policy", "terms-of-service"} {
		tabName := tab
		mux.HandleFunc("GET /"+tabName, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			http.ServeFileFS(w, r, subFS, "index.html")
		})
	}

	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	}))

	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	clientConfigured := s.service.clientID != "" && s.service.clientSecret != ""
	oauthConnected := false
	userEmail := ""
	userName := ""
	userPic := ""

	token, err := GetSessionCookie(r)
	if err == nil && token != nil {
		client, err := s.service.GetOAuthClient(r.Context(), token, w)
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
			ClearSessionCookie(w)
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
		UserEmail:        userEmail,
		UserName:         userName,
		UserPicture:      userPic,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAuthURL(w http.ResponseWriter, r *http.Request) {
	if s.service.clientID == "" || s.service.clientSecret == "" {
		http.Error(w, "OAuth Client ID or Secret not configured on server", http.StatusInternalServerError)
		return
	}

	config := s.service.GetOAuthConfig(s.redirectURI)
	reqScope := r.URL.Query().Get("scope")
	if reqScope != "" {
		scopeURI := reqScope
		switch reqScope {
		case "sleep":
			scopeURI = "https://www.googleapis.com/auth/googlehealth.sleep.readonly"
		case "heart":
			scopeURI = "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly"
		case "activity":
			scopeURI = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly"
		}
		config.Scopes = []string{
			scopeURI,
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		}
	}

	authURL := config.AuthCodeURL("state-token",
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error=no_code", http.StatusFound)
		return
	}

	if s.service.clientID == "" || s.service.clientSecret == "" {
		http.Redirect(w, r, "/?error=server_not_configured", http.StatusFound)
		return
	}

	config := s.service.GetOAuthConfig(s.redirectURI)
	token, err := config.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("[ERROR] OAuth token exchange failed: %v", err)
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}

	if err := SetSessionCookie(w, token); err != nil {
		log.Printf("[ERROR] Failed to set session cookie: %v", err)
		http.Redirect(w, r, "/?error=cookie_error", http.StatusFound)
		return
	}

	log.Printf("✅ OAuth Exchange successful and session cookie set!")
	http.Redirect(w, r, "/?connected=true", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	ClearSessionCookie(w)
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	token, err := GetSessionCookie(r)
	if err != nil || token == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GenerateSampleStats(14))
		return
	}

	client, err := s.service.GetOAuthClient(r.Context(), token, w)
	if err != nil {
		ClearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GenerateSampleStats(14))
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

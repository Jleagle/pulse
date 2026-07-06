package main

import (
	"embed"
	"encoding/json"
	"io/fs"
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

	// Legal & OAuth compliance pages
	mux.HandleFunc("GET /privacy-policy", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, subFS, "privacy-policy.html")
	})
	mux.HandleFunc("GET /terms-of-service", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, subFS, "terms-of-service.html")
	})

	mux.Handle("/", http.FileServer(http.FS(subFS)))

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
			profile, err := s.service.FetchUserProfile(client)
			if err == nil && profile != nil {
				oauthConnected = true
				userEmail = profile.Email
				userName = profile.Name
				userPic = profile.Picture
			} else {
				// Token might be invalid or revoked, clear cookie
				ClearSessionCookie(w)
			}
		} else {
			ClearSessionCookie(w)
		}
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

	config := &oauth2.Config{
		ClientID:     s.service.clientID,
		ClientSecret: s.service.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL: "https://accounts.google.com/o/oauth2/auth",
		},
		RedirectURL: s.redirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/googlehealth.sleep.readonly",
			"https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly",
			"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly",
			"https://www.googleapis.com/auth/googlehealth.profile.readonly",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error=no_code", http.StatusTemporaryRedirect)
		return
	}

	if s.service.clientID == "" || s.service.clientSecret == "" {
		http.Redirect(w, r, "/?error=server_not_configured", http.StatusTemporaryRedirect)
		return
	}

	config := &oauth2.Config{
		ClientID:     s.service.clientID,
		ClientSecret: s.service.clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		RedirectURL: s.redirectURI,
	}

	token, err := config.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusTemporaryRedirect)
		return
	}

	if err := SetSessionCookie(w, token); err != nil {
		http.Redirect(w, r, "/?error=cookie_error", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/?connected=true", http.StatusTemporaryRedirect)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	ClearSessionCookie(w)
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	token, err := GetSessionCookie(r)
	if err != nil || token == nil {
		http.Error(w, "Unauthorized: No valid session", http.StatusUnauthorized)
		return
	}

	client, err := s.service.GetOAuthClient(r.Context(), token, w)
	if err != nil {
		ClearSessionCookie(w)
		http.Error(w, "Unauthorized: Failed to authenticate with Google", http.StatusUnauthorized)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	days := 14 // Default to 14 days of live data for fast page load
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 90 {
			days = l
		}
	}

	stats, err := s.service.FetchAllStats(client, days)
	if err != nil {
		http.Error(w, "Failed to fetch health stats from Google API: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

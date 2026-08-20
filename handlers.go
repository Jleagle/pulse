package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	mux.HandleFunc("POST /api/security-events", s.handleSecurityEvents)
	mux.HandleFunc("POST /api/config", s.handleUpdateConfig)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)

	// Frontend static assets
	subFS, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err)
	}

	// SPA tab routing so page refreshes and links work cleanly
	for _, tab := range []string{"overview", "sleep", "heart", "activity", "settings", "setup-guide", "privacy-policy", "terms-of-service"} {
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

func (s *Server) effectiveRedirectURI(r *http.Request) string {
	_, _, configuredRedirect := s.service.GetCredentials()
	if configuredRedirect != "" && !strings.Contains(configuredRedirect, "localhost") {
		return configuredRedirect
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
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

	masked := ""
	if len(cID) > 12 {
		masked = cID[:8] + "..." + cID[len(cID)-8:]
	} else if len(cID) > 0 {
		masked = "********"
	}

	resp := StatusResponse{
		ClientConfigured: clientConfigured,
		OAuthConnected:   oauthConnected,
		UserEmail:        userEmail,
		UserName:         userName,
		UserPicture:      userPic,
		RedirectURI:      s.effectiveRedirectURI(r),
		ClientIDMasked:   masked,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cID, _, rURI := s.service.GetCredentials()
	if rURI == "" {
		rURI = s.effectiveRedirectURI(r)
	}
	masked := ""
	if len(cID) > 12 {
		masked = cID[:8] + "..." + cID[len(cID)-8:]
	} else if len(cID) > 0 {
		masked = "********"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"client_id_masked": masked,
		"redirect_uri":     rURI,
		"configured":       strconv.FormatBool(cID != ""),
	})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ClientID) == "" || strings.TrimSpace(req.ClientSecret) == "" {
		http.Error(w, "Both Client ID and Client Secret are required", http.StatusBadRequest)
		return
	}
	err := s.service.SetCredentials(req.ClientID, req.ClientSecret, req.RedirectURI)
	if err != nil {
		http.Error(w, "Failed to save configuration: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] Self-Hosted OAuth Client credentials updated via Web UI")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Status:      "ok",
		Message:     "Credentials saved and activated successfully!",
		RedirectURI: s.effectiveRedirectURI(r),
	})
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

// handleSecurityEvents implements Google Cross-Account Protection (RISC - RFC 8915) Security Event Token receiver
func (s *Server) handleSecurityEvents(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Bad Request: empty or unreadable SET body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	parts := strings.Split(string(body), ".")
	if len(parts) == 3 {
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			payloadBytes, _ = base64.URLEncoding.DecodeString(parts[1])
		}
		if len(payloadBytes) > 0 {
			var setPayload map[string]interface{}
			if json.Unmarshal(payloadBytes, &setPayload) == nil {
				log.Printf("[SECURITY EVENT TOKEN] Google Cross-Account Protection alert received: iss=%v sub=%v events=%v",
					setPayload["iss"], setPayload["sub"], setPayload["events"])
			}
		}
	} else {
		log.Printf("[SECURITY EVENT TOKEN] Received non-JWT or raw RISC event notification (%d bytes)", len(body))
	}

	w.WriteHeader(http.StatusAccepted)
}

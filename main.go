package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	redirectURL := os.Getenv("REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("http://localhost:%s/oauth/callback", port)
	}

	// 1. Initialize Stateless Service (auto-loads config.json if present)
	service := NewStatelessService(clientID, clientSecret, redirectURL)

	cID, _, _ := service.GetCredentials()
	if cID == "" {
		log.Println("[INFO] ⚙️  No OAuth Client credentials in environment or config.json.")
		log.Println("[INFO] 👉 Open the Web UI in your browser to configure Self-Hosted Google OAuth keys directly!")
	} else {
		log.Println("[INFO] ✅ Google OAuth Client credentials loaded and active.")
	}

	// 2. Initialize HTTP Server with routes
	server := NewServer(service, redirectURL)
	handler := server.Routes()

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Starting Pulse Self-Hosted Server on %s", addr)
	log.Printf("🔗 Default OAuth Callback URI: %s (Dynamically adjusts to accessed host)", redirectURL)
	log.Println("⚡ Running in 100% Stateless & Private Mode (No database required)")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

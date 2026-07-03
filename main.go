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

	if clientID == "" || clientSecret == "" {
		log.Println("[WARNING] CLIENT_ID or CLIENT_SECRET environment variables are not set. Google OAuth authentication will not work until configured.")
	} else {
		log.Println("[INFO] Google OAuth Client credentials successfully loaded from environment.")
	}

	// 1. Initialize Stateless Service
	service := NewStatelessService(clientID, clientSecret, redirectURL)

	// 2. Initialize HTTP Server with routes
	server := NewServer(service, redirectURL)
	handler := server.Routes()

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Starting Pulse Stateless Server on %s", addr)
	log.Printf("🔗 OAuth Callback URI configured as: %s", redirectURL)
	log.Println("⚡ Running in 100% Stateless Mode (No database or local disk storage)")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

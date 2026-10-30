package main

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := cmp.Or(os.Getenv("PORT"), os.Getenv("PULSE_PORT"), "8080")
	clientID := cmp.Or(os.Getenv("CLIENT_ID"), os.Getenv("PULSE_CLIENT_ID"))
	clientSecret := cmp.Or(os.Getenv("CLIENT_SECRET"), os.Getenv("PULSE_CLIENT_SECRET"))
	redirectURL := cmp.Or(
		os.Getenv("REDIRECT_URL"),
		os.Getenv("PULSE_REDIRECT_URL"),
		fmt.Sprintf("http://localhost:%s/oauth/callback", port),
	)

	// 1. Initialize Stateless Service from environment variables
	service := NewStatelessService(clientID, clientSecret, redirectURL)

	cID, cSec, _ := service.GetCredentials()
	if cID == "" || cSec == "" {
		log.Println("[WARNING] ⚠️  CLIENT_ID or CLIENT_SECRET environment variables are not set.")
		log.Println("[WARNING] 👉 Set CLIENT_ID and CLIENT_SECRET in your environment to enable Google OAuth login.")
	} else {
		log.Println("[INFO] ✅ Google OAuth Client credentials loaded from environment.")
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

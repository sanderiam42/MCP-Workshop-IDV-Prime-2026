package main

import (
	"log"
	"net/http"
	"os"

	"xaa-mcp-demo/internal/authserver"
	"xaa-mcp-demo/internal/shared/debuglog"
)

func main() {
	port := envOrDefault("PORT", "8081")
	dataDir := envOrDefault("DATA_DIR", "./data")
	issuer := envOrDefault("AUTH_SERVER_ISSUER", "http://localhost:8081")
	verbose := envOrDefault("VERBOSE", "") == "true"
	logFile := envOrDefault("LOG_FILE", "")

	logger, err := debuglog.New("auth-server", verbose, logFile)
	if err != nil {
		log.Fatalf("create logger: %v", err)
	}

	service, err := authserver.NewService(dataDir, issuer, logger)
	if err != nil {
		log.Fatalf("create auth server: %v", err)
	}

	log.Printf("auth server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, service.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

package main

import (
	"log"
	"net/http"
	"os"

	"xaa-mcp-demo/internal/resourceserver"
	"xaa-mcp-demo/internal/shared/debuglog"
)

func main() {
	port := envOrDefault("PORT", "8082")
	dataDir := envOrDefault("DATA_DIR", "./data")
	issuer := envOrDefault("RESOURCE_SERVER_ISSUER", "http://localhost:8082")
	authIssuer := envOrDefault("AUTH_SERVER_ISSUER", "http://localhost:8081")
	authJWKSURL := envOrDefault("AUTH_SERVER_JWKS_URL", authIssuer+"/oauth/jwks.json")
	verbose := envOrDefault("VERBOSE", "") == "true"
	logFile := envOrDefault("LOG_FILE", "")

	logger, err := debuglog.New("resource-server", verbose, logFile)
	if err != nil {
		log.Fatalf("create logger: %v", err)
	}

	service, err := resourceserver.NewService(dataDir, issuer, authIssuer, authJWKSURL, logger)
	if err != nil {
		log.Fatalf("create resource server: %v", err)
	}

	log.Printf("resource server listening on :%s", port)
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

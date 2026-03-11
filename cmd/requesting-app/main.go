package main

import (
	"log"
	"net/http"
	"os"

	"xaa-mcp-demo/internal/requestingapp"
	"xaa-mcp-demo/internal/shared/debuglog"
)

func main() {
	port := envOrDefault("PORT", "3000")
	dataDir := envOrDefault("DATA_DIR", "./data")
	webDir := envOrDefault("WEB_DIST_DIR", "./web/dist")
	publicBase := envOrDefault("REQUESTING_APP_BASE_URL", "http://localhost:3000")
	authPublicBase := envOrDefault("AUTH_SERVER_ISSUER", "http://localhost:8081")
	authInternalBase := envOrDefault("AUTH_SERVER_INTERNAL_URL", "http://localhost:8081")
	resourcePublicBase := envOrDefault("RESOURCE_SERVER_ISSUER", "http://localhost:8082")
	resourceInternalBase := envOrDefault("RESOURCE_SERVER_INTERNAL_URL", "http://localhost:8082")
	verbose := envOrDefault("VERBOSE", "") == "true"
	logFile := envOrDefault("LOG_FILE", "")

	logger, err := debuglog.New("requesting-app", verbose, logFile)
	if err != nil {
		log.Fatalf("create logger: %v", err)
	}

	service := requestingapp.NewService(
		dataDir,
		webDir,
		publicBase,
		authPublicBase,
		authInternalBase,
		resourcePublicBase,
		resourceInternalBase,
		logger,
	)

	log.Printf("requesting app listening on :%s", port)
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

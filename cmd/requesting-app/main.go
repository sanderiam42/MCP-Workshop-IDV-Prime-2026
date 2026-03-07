package main

import (
	"log"
	"net/http"
	"os"

	"xaa-mcp-demo/internal/requestingapp"
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

	service := requestingapp.NewService(
		dataDir,
		webDir,
		publicBase,
		authPublicBase,
		authInternalBase,
		resourcePublicBase,
		resourceInternalBase,
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

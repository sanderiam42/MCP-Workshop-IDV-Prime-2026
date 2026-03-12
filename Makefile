.PHONY: test web-install web-build build build-stdio-mac build-stdio-linux up down reset-state

test:
	cd xaa-demo && go test ./...

web-install:
	npm install --prefix xaa-demo/web

web-build: web-install
	npm run build --prefix xaa-demo/web

build: web-build
	cd xaa-demo && go build ./...

build-stdio-mac:
	cd xaa-demo && go build -o ../bin/xaa-mcp-stdio-darwin ./cmd/xaa-mcp-stdio

build-stdio-linux:
	cd xaa-demo && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	  go build -o ../bin/xaa-mcp-stdio-linux-amd64 ./cmd/xaa-mcp-stdio

up:
	docker compose up --build

down:
	docker compose down

reset-state:
	rm -f data/auth/auth-state.json
	rm -f data/resource/resource-state.json
	rm -f data/requesting-app/requesting-app-state.json

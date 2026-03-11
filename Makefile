.PHONY: test web-install web-build build up down reset-state

test:
	go test ./...

web-install:
	npm install --prefix web

web-build: web-install
	npm run build --prefix web

build: web-build
	go build ./...

up:
	docker compose up --build

down:
	docker compose down

reset-state:
	rm -f data/auth/auth-state.json
	rm -f data/resource/resource-state.json
	rm -f data/requesting-app/requesting-app-state.json

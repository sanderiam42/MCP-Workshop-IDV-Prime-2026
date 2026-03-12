.PHONY: test web-install web-build build up down reset-state

test:
	cd xaa-demo && go test ./...

web-install:
	npm install --prefix xaa-demo/web

web-build: web-install
	npm run build --prefix xaa-demo/web

build: web-build
	cd xaa-demo && go build ./...

up:
	docker compose --env-file xaa-demo/.env up --build

down:
	docker compose --env-file xaa-demo/.env down

reset-state:
	rm -f data/auth/auth-state.json
	rm -f data/resource/resource-state.json
	rm -f data/requesting-app/requesting-app-state.json

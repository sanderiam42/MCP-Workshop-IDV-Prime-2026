FROM node:20-alpine AS web-build

WORKDIR /web

COPY web/package.json ./package.json
COPY web/tsconfig.json ./tsconfig.json
COPY web/vite.config.ts ./vite.config.ts
COPY web/index.html ./index.html
COPY web/src ./src

RUN npm install --no-fund --no-audit && npm run build

FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/requesting-app ./cmd/requesting-app

FROM alpine:3.21

WORKDIR /app

COPY --from=build /out/requesting-app /app/requesting-app
COPY --from=web-build /web/dist /app/web/dist

RUN mkdir -p /app/data /app/web/dist

EXPOSE 3000

CMD ["/app/requesting-app"]

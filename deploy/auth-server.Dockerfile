FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/auth-server ./cmd/auth-server

FROM alpine:3.21

WORKDIR /app

COPY --from=build /out/auth-server /app/auth-server

RUN mkdir -p /app/data

EXPOSE 8081

CMD ["/app/auth-server"]

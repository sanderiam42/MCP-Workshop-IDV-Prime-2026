FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/resource-server ./cmd/resource-server

FROM alpine:3.21

WORKDIR /app

COPY --from=build /out/resource-server /app/resource-server

RUN mkdir -p /app/data

EXPOSE 8082

CMD ["/app/resource-server"]

FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o search-service main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/search-service ./search-service
EXPOSE 8080
ENTRYPOINT ["./search-service"]
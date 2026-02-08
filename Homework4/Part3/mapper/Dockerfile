FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY mapper.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o mapper mapper.go

FROM scratch
COPY --from=builder /app/mapper /mapper
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/mapper"]
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY splitter.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o splitter splitter.go

FROM scratch
COPY --from=builder /app/splitter /splitter
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/splitter"]
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY reducer.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o reducer reducer.go

FROM scratch
COPY --from=builder /app/reducer /reducer
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/reducer"]
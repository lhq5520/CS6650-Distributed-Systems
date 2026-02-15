# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY src/go.mod .
RUN go mod download
COPY src/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Run stage
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 5173
CMD ["./server"]
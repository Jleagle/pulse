# Step 1: Build the Go binary
FROM golang:1.26-alpine AS builder

# Install system dependencies (git/certs)
RUN apk update && apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build pure-Go binary without CGO, static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pulse .

# Step 2: Create a minimal stateless production runner image
FROM alpine:3.19

# Install ca-certificates to connect to Google APIs over HTTPS and tzdata for dates
RUN apk update && apk add --no-cache ca-certificates tzdata && \
    addgroup -S pulse && adduser -S -G pulse pulse

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/pulse /app/pulse

USER pulse

# Expose default HTTP port
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/status || exit 1

# Run the stateless binary
CMD ["/app/pulse"]

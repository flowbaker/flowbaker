# Build stage
FROM golang:1.25.0-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG GIT_COMMIT=""
ARG BUILD_DATE=""
ARG BUILD_USER=""

# Install git and ca-certificates for dependency fetching
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with version information
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X github.com/flowbaker/flowbaker/internal/version.Version=${VERSION} \
              -X github.com/flowbaker/flowbaker/internal/version.GitCommit=${GIT_COMMIT} \
              -X github.com/flowbaker/flowbaker/internal/version.BuildDate=${BUILD_DATE} \
              -X github.com/flowbaker/flowbaker/internal/version.BuildUser=${BUILD_USER}" \
    -o flowbaker ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create app directory and non-root user
RUN mkdir /app && \
    addgroup -g 1001 flowbaker && \
    adduser -D -u 1001 -G flowbaker flowbaker

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/flowbaker .

# Change ownership to non-root user
RUN chown -R flowbaker:flowbaker /app

# Switch to non-root user
USER flowbaker

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Default command
CMD ["./flowbaker", "start"]

# ---- Build-Stage ----------------------------------------------------------
FROM golang:1.22-alpine AS build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Clone the repository
RUN git clone --depth 1 https://github.com/Robert-Langbein/go-dispatch-proxy-enhanced.git .

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o go-dispatch-proxy-enhanced .

# ---- Runtime-Stage --------------------------------------------------------
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite iptables iproute2 wget

# Create app directory and user
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

# Create directories
RUN mkdir -p /app/data /app/web && \
    chown -R appuser:appgroup /app

# Copy binary from build stage
COPY --from=build /app/go-dispatch-proxy-enhanced /app/go-dispatch-proxy-enhanced

# Copy web assets
COPY --from=build /app/web /app/web

# Set working directory
WORKDIR /app

# Use non-root user for security
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8095/login || exit 1

# Default command - ports are configurable within the application
ENTRYPOINT ["./go-dispatch-proxy-enhanced"]
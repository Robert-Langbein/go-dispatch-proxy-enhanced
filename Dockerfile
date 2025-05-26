# ---- Build-Stage ----------------------------------------------------------
    FROM golang:1.22-alpine AS build

    # Install build dependencies
    RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev
    
    # Set working directory
    WORKDIR /app
    
    # Clone the repository
    RUN git clone --depth 1 --single-branch --branch sqlite-integration https://github.com/Robert-Langbein/go-dispatch-proxy-enhanced.git .
    
    # Download dependencies
    RUN go mod download
    
    # Build the application
    RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o go-dispatch-proxy-enhanced .
    #RUN go build -o /go-dispatch-proxy-enhanced .
    
    # ---- Runtime-Stage --------------------------------------------------------
    FROM alpine:latest
    
    # Install runtime dependencies
    RUN apk add --no-cache ca-certificates sqlite iptables iproute2 wget
    
    # Create directories with proper permissions
    RUN mkdir -p /app/data /app/web /app/logs
    
    # Copy binary from build stage
    COPY --from=build /app/go-dispatch-proxy-enhanced /app/go-dispatch-proxy-enhanced
    
    # Copy web assets
    COPY --from=build /app/web /app/web
    
    # Set working directory
    WORKDIR /app
    
    # Make binary executable
    RUN chmod +x /app/go-dispatch-proxy-enhanced
    
    # Run as root for Gateway mode (iptables, routing, network interfaces)
    
    # Default command - ports are configurable within the application
ENTRYPOINT ["./go-dispatch-proxy-enhanced"]
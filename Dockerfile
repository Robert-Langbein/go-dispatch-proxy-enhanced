# ---- Build-Stage ----------------------------------------------------------
    FROM golang:1.22-alpine AS build
    WORKDIR /src
    RUN apk add --no-cache git
    # schlanker Shallow-Clone
    RUN git clone --depth 1 https://github.com/Robert-Langbein/go-dispatch-proxy-enhanced.git .
    RUN go build -o /go-dispatch-proxy .
    
    # ---- Runtime-Stage --------------------------------------------------------
    FROM alpine:latest
    COPY --from=build /go-dispatch-proxy /usr/local/bin/go-dispatch-proxy
    EXPOSE 33333/tcp
    EXPOSE 8090/tcp
    
    COPY entrypoint.sh /entrypoint.sh
    RUN chmod +x /entrypoint.sh
    ENTRYPOINT ["/entrypoint.sh"]
    # ENTRYPOINT ["go-dispatch-proxy"]
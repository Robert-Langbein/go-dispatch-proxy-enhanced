# ---- Build-Stage ----------------------------------------------------------
ARG WEBGUI_PORT=8090
ARG PROXY_PORT=33333

FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
# schlanker Shallow-Clone
RUN git clone --depth 1 https://github.com/Robert-Langbein/go-dispatch-proxy-enhanced.git .
RUN go build -o /go-dispatch-proxy .

# ---- Runtime-Stage --------------------------------------------------------
FROM alpine:latest
COPY --from=build /go-dispatch-proxy /usr/local/bin/go-dispatch-proxy
COPY --from=build /src/web /web
EXPOSE ${PROXY_PORT}/tcp
EXPOSE ${WEBGUI_PORT}/tcp

ENTRYPOINT ["go-dispatch-proxy-enhanced", "-webgui", "${WEBGUI_PORT}"]
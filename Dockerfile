# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/drone-tools ./cmd/drone-tools

FROM alpine:3.22
RUN addgroup -S -g 10001 drone-tools && \
    adduser -S -D -H -u 10001 -G drone-tools drone-tools && \
    mkdir -p /data && chown drone-tools:drone-tools /data
COPY --from=build /out/drone-tools /usr/local/bin/drone-tools
COPY --chmod=755 scripts/healthcheck.sh /usr/local/bin/drone-tools-healthcheck
USER 10001:10001
VOLUME ["/data"]
EXPOSE 8080
ENV DRONE_TOOLS_ADDR=:8080 \
    DRONE_TOOLS_DATA_DIR=/data
ENTRYPOINT ["/usr/local/bin/drone-tools"]

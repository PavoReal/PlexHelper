FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl
COPY plex-helper /usr/local/bin/plex-helper
COPY config.json /etc/plex-helper/config.json
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1
ENTRYPOINT ["plex-helper", "-config", "/etc/plex-helper/config.json"]

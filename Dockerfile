FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY plex-helper /usr/local/bin/plex-helper
COPY config.json /etc/plex-helper/config.json
ENTRYPOINT ["plex-helper", "-config", "/etc/plex-helper/config.json"]

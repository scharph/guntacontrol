# --- web UI ----------------------------------------------------------------
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npx ng build

# --- server ------------------------------------------------------------------
FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Bring in the freshly built UI so //go:embed in web/embed.go finds dist/web/browser.
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/guntacontrol ./cmd/guntacontrol

# --- runtime -----------------------------------------------------------------
FROM alpine:3.22
RUN apk add --no-cache tzdata ca-certificates su-exec && adduser -D -u 1000 gunta
WORKDIR /data
COPY --from=server /out/guntacontrol /usr/local/bin/guntacontrol
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENV TZ=Europe/Vienna
EXPOSE 8080
VOLUME /data
# starts as root only to fix /data ownership, then drops to user "gunta".
# The UI is embedded in the binary (no -webroot); history + audit live in SQLite
# under /data. All logs go to stdout — no local log files are written.
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["guntacontrol", "-addr", ":8080", "-db", "/data/guntacontrol.db", "-state", "/data/state.json"]

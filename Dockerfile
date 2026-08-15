# The repository layout is preserved here because Vite writes its output to
# internal/web/dist, one level above the web/ directory it builds from.
FROM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci
COPY web/ ./web/
RUN npm --prefix web run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/dist ./internal/web/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
# The shared TMDB key; a build without it works, operators enter their own.
ARG TMDB_KEY=""
ENV CGO_ENABLED=0
RUN go build -trimpath \
      -ldflags "-s -w \
        -X github.com/sirrobot01/snagarr/internal/version.Version=${VERSION} \
        -X github.com/sirrobot01/snagarr/internal/version.Commit=${COMMIT} \
        -X github.com/sirrobot01/snagarr/internal/version.Date=${DATE} \
        -X github.com/sirrobot01/snagarr/internal/config.DefaultTMDBKey=${TMDB_KEY}" \
      -o /snagarr ./cmd/snagarr

# Alpine rather than distroless: PUID/PGID needs a shell, a user database and
# su-exec to drop privileges, and distroless has none of them.
FROM alpine:3
RUN apk add --no-cache ca-certificates su-exec tzdata
COPY --from=build /snagarr /usr/bin/snagarr
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
VOLUME /data
ENV SNAGARR_DATA_DIR=/data \
    SNAGARR_ADDR=:8080 \
    PUID=1000 \
    PGID=1000 \
    UMASK=022
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${SNAGARR_ADDR##*:}/api/v1/health" >/dev/null || exit 1
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/snagarr"]

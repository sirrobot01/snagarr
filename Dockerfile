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
ENV CGO_ENABLED=0
RUN go build -trimpath \
      -ldflags "-s -w \
        -X github.com/sirrobot01/snagarr/internal/version.Version=${VERSION} \
        -X github.com/sirrobot01/snagarr/internal/version.Commit=${COMMIT} \
        -X github.com/sirrobot01/snagarr/internal/version.Date=${DATE}" \
      -o /snagarr ./cmd/snagarr

FROM gcr.io/distroless/static:nonroot
COPY --from=build /snagarr /snagarr
USER nonroot:nonroot
EXPOSE 8080
VOLUME /data
ENV SNAGARR_DATA_DIR=/data
ENTRYPOINT ["/snagarr"]

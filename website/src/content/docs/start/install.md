---
title: Install
description: Run Snagarr with Docker, a release binary or systemd, then back it up and upgrade it.
---

## Docker Compose

```yaml
services:
  snagarr:
    image: ghcr.io/sirrobot01/snagarr:latest
    ports: ["8080:8080"]
    volumes:
      - snagarr:/data
    restart: unless-stopped
volumes:
  snagarr:
```

```sh
docker compose up -d
docker compose logs snagarr   # prints the admin token once
```

Continue at [First run](/snagarr/start/first-run/).

### Bind mount

The image runs as `nonroot`, UID 65532. A bind mount must be writable by that UID.

1. Make the directory: `mkdir -p ./data`.
2. Give it to the container user: `sudo chown -R 65532:65532 ./data`.
3. Replace the volume line with `- ./data:/data`.

### Pin settings from the environment

```yaml
    environment:
      SNAGARR_TMDB_API_KEY: your_tmdb_key
      SNAGARR_RADARR_URL: http://radarr.lan:7878
      SNAGARR_RADARR_API_KEY: your_radarr_key
```

`SNAGARR_TMDB_API_KEY` pins a setting and locks the TMDB card. `SNAGARR_RADARR_*` writes a Radarr [service](/snagarr/configure/services/) owned by the **first admin**, named `Default`, rewritten on every start and rendered read-only.

Every other member connects their own services in the UI. No variable touches them. See [Environment variables](/snagarr/configure/environment/).

## Plain Docker

```sh
docker volume create snagarr

docker run -d \
  --name snagarr \
  --restart unless-stopped \
  -p 8080:8080 \
  -v snagarr:/data \
  ghcr.io/sirrobot01/snagarr:latest
```

## Images

| Registry | Image |
|----------|-------|
| GitHub | `ghcr.io/sirrobot01/snagarr` |
| Docker Hub | `docker.io/sirrobot01/snagarr` |

| Tag | Example | Moves |
|-----|---------|-------|
| `latest` | `snagarr:latest` | On every release |
| `<major>.<minor>` | `snagarr:0.1` | On every patch release of that line |
| `<version>` | `snagarr:0.1.0` | Never |

Platforms: `linux/amd64` and `linux/arm64`. Cosign signs the images keylessly.

```sh
cosign verify ghcr.io/sirrobot01/snagarr:0.1.0 \
  --certificate-identity-regexp 'https://github\.com/sirrobot01/snagarr/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Binary

Builds exist for Linux, macOS, Windows and FreeBSD on `amd64`, `arm64` and `arm` v7. Archive names follow `snagarr_<version>_<os>_<arch>.tar.gz`; Windows uses `.zip` and `arm` adds the version, for example `snagarr_0.1.0_linux_armv7.tar.gz`.

1. Download the archive and `snagarr_<version>_checksums.txt` from the [releases page](https://github.com/sirrobot01/snagarr/releases).
2. Check the archive: `sha256sum --check --ignore-missing snagarr_<version>_checksums.txt`.
3. Unpack it: `tar xzf snagarr_<version>_linux_amd64.tar.gz`.
4. Install it: `sudo install -m 0755 snagarr /usr/local/bin/snagarr`.

Verify the checksum file against the cosign bundle beside it:

```sh
cosign verify-blob snagarr_0.1.0_checksums.txt \
  --bundle snagarr_0.1.0_checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github\.com/sirrobot01/snagarr/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

### Commands

```sh
snagarr serve --addr :8080 --data /var/lib/snagarr
snagarr version    # version, commit, build date
```

`serve` is the default command. `snagarr` alone does the same.

### systemd

Write `/etc/systemd/system/snagarr.service`:

```ini
[Unit]
Description=Snagarr
After=network-online.target
Wants=network-online.target

[Service]
User=snagarr
Group=snagarr
Environment=SNAGARR_DATA_DIR=/var/lib/snagarr
ExecStart=/usr/local/bin/snagarr serve
Restart=on-failure
StateDirectory=snagarr

[Install]
WantedBy=multi-user.target
```

1. Add the user: `sudo useradd --system --no-create-home snagarr`.
2. Reload systemd: `sudo systemctl daemon-reload`.
3. Start the service: `sudo systemctl enable --now snagarr`.
4. Read the admin token: `sudo journalctl -u snagarr`.

## Files on disk

| File | Contents |
|------|----------|
| `snagarr.db` | Items, users, tokens, services, indexes and settings |
| `snagarr.db-wal`, `snagarr.db-shm` | SQLite write-ahead log |
| `secret.key` | The 32-byte key that encrypts the stored settings and service configs, mode `0600` |

Mount one volume at the data directory. The default is `data` beside the working directory; the image sets `/data`.

## Port

| `SNAGARR_ADDR` | Effect |
|----------------|--------|
| `:8080` | Every interface, port 8080 (default) |
| `127.0.0.1:8080` | Loopback only |
| `0.0.0.0:9090` | Every interface, port 9090 |

Snagarr serves plain HTTP and has no TLS options.

## Reverse proxy

1. Terminate TLS at the proxy.
2. Forward `/` to `http://snagarr:8080`.
3. Set `SNAGARR_PUBLIC_URL` to the external URL, for example `https://snagarr.example.com`.

Three constraints:

- **Use a host, not a subpath.** The web client calls `/api/v1` from the domain root. A proxy that mounts Snagarr under `/snagarr/` breaks it.
- **Leave `/api/v1/webhooks/` unauthenticated.** Senders authenticate with a query parameter, not a header. See [Webhooks](/snagarr/use/webhooks/).
- **Add no CORS headers at the proxy.** Snagarr sends its own on every `/api/v1` route. A duplicate `Access-Control-Allow-Origin` makes the browser reject the response.

Snagarr answers preflight `OPTIONS` with `204`, allows any origin and allows the `Authorization` and `Content-Type` headers. It authenticates with a bearer token, never a cookie. It reads no proxy header.

### Caddy

```text title="Caddyfile"
snagarr.example.com {
    reverse_proxy snagarr:8080
}
```

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name snagarr.example.com;

    location / {
        proxy_pass http://snagarr:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Health check

```sh
curl http://localhost:8080/api/v1/health
# {"status":"ok","version":"0.1.0"}
```

`GET /api/v1/health` needs no token. The image is distroless and holds no shell and no `curl`, so an in-container `HEALTHCHECK` is not possible. Probe the endpoint from outside.

## Backup

Back up `snagarr.db` and `secret.key` together. Snagarr cannot read a stored setting or service config without `secret.key`: a database restored beside a new key loses every API key and every server token.

The database uses write-ahead logging, so a live copy of `snagarr.db` alone can be incomplete. Stop the process first.

```sh
docker compose stop snagarr
docker run --rm -v snagarr:/data -v "$PWD":/backup alpine \
  tar czf /backup/snagarr-backup.tar.gz -C /data .
docker compose start snagarr
```

### Restore

1. Stop Snagarr.
2. Put `snagarr.db` and `secret.key` back in the data directory.
3. Delete any `snagarr.db-wal` and `snagarr.db-shm` from an older copy.
4. Start Snagarr.

Tokens survive a restore. The indexes rebuild on the first reconcile pass.

## Upgrade

Migrations run at start-up and move forward only. Back up before you upgrade; a downgrade needs the old database file.

```sh
docker compose pull && docker compose up -d
```

With the binary:

1. Stop the service: `sudo systemctl stop snagarr`.
2. Replace the binary: `sudo install -m 0755 snagarr /usr/local/bin/snagarr`.
3. Start the service: `sudo systemctl start snagarr`.
4. Check the version: `curl -s http://localhost:8080/api/v1/health`.

## Build from source

Requires Go, Node 22 and [Task](https://taskfile.dev).

```sh
task install   # Go tools and web dependencies
task build     # builds the UI, then bin/snagarr
```

`task build` writes the React client to `internal/web/dist`, which the Go build embeds. A binary built without that step serves the API and returns 404 for the UI.

| Task | Effect |
|------|--------|
| `task web` | Builds the React client only |
| `task dev` | API with live reload, plus the Vite dev server |
| `task test` | Go tests and web tests |
| `task lint` | golangci-lint and a formatting check |
| `task docker` | Builds the local image from `Dockerfile` |
| `task snapshot` | Builds an unpublished release with GoReleaser |
| `task clean` | Removes the build output |

`docker build -t snagarr:dev .` builds the client and the binary from source. `Dockerfile.release` is for GoReleaser only; it copies binaries GoReleaser already built.

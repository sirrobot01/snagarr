# Deployment

Snagarr is one Go binary. It embeds the web client. It stores everything in one
SQLite file.

It needs no external database. It needs no cache server. It needs no reverse
proxy.

## What Snagarr writes

Everything lives in the data directory.

| File | What it holds |
|------|---------------|
| `snagarr.db` | The database: items, users, tokens, indexes and settings. |
| `snagarr.db-wal`, `snagarr.db-shm` | The SQLite write-ahead log. |
| `secret.key` | The 32-byte key that encrypts the stored settings. |

The default data directory is `data`, next to the working directory. The Docker
image sets it to `/data`.

Mount one volume at the data directory. Back up all four files together. See
[Backup](#backup).

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

Start it:

```sh
docker compose up -d
```

Then read the first-run token from the logs. See [First run](#first-run).

### Bind mount instead of a named volume

The image runs as the `nonroot` user of the distroless base image, which is UID
65532. A bind mount must be writable by that user.

1. Make the directory: `mkdir -p ./data`.
2. Give it to the container user: `sudo chown -R 65532:65532 ./data`.
3. Replace the volume line with `- ./data:/data`.

### Environment variables

Add the variables you want to pin under an `environment:` key:

```yaml
services:
  snagarr:
    image: ghcr.io/sirrobot01/snagarr:latest
    ports: ["8080:8080"]
    volumes:
      - snagarr:/data
    environment:
      SNAGARR_TMDB_API_KEY: your_tmdb_key
      SNAGARR_RADARR_URL: http://radarr.lan:7878
      SNAGARR_RADARR_API_KEY: your_radarr_key
    restart: unless-stopped
volumes:
  snagarr:
```

Each variable locks its settings card in the UI. See
[configuration.md](configuration.md).

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

Three tags point at each release:

| Tag | Example | Moves |
|-----|---------|-------|
| `latest` | `snagarr:latest` | On every release. |
| `<major>.<minor>` | `snagarr:0.1` | On every patch release of that line. |
| `<version>` | `snagarr:0.1.0` | Never. |

Pin the full version for a stable deployment.

The images are built for `linux/amd64` and `linux/arm64`. Cosign signs them
keylessly.

Verify an image:

```sh
cosign verify ghcr.io/sirrobot01/snagarr:0.1.0 \
  --certificate-identity-regexp 'https://github\.com/sirrobot01/snagarr/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Binary

Every release publishes an archive per platform. The archive holds the binary,
the licence, the README and the `docs` directory.

Builds exist for Linux, macOS, Windows and FreeBSD, on `amd64`, `arm64` and
`arm` v7.

1. Download the archive for your platform from the
   [releases page](https://github.com/sirrobot01/snagarr/releases). The name is
   `snagarr_<version>_<os>_<arch>.tar.gz`.
2. Download `snagarr_<version>_checksums.txt`.
3. Check the archive: `sha256sum --check --ignore-missing snagarr_<version>_checksums.txt`.
4. Unpack it: `tar xzf snagarr_<version>_linux_amd64.tar.gz`.
5. Install it: `sudo install -m 0755 snagarr /usr/local/bin/snagarr`.

The checksum file is signed with cosign. Verify it with the bundle beside it:

```sh
cosign verify-blob snagarr_0.1.0_checksums.txt \
  --bundle snagarr_0.1.0_checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github\.com/sirrobot01/snagarr/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

### Run the binary

```sh
snagarr serve --addr :8080 --data /var/lib/snagarr
```

`serve` is the default command. `snagarr` on its own does the same thing.

`snagarr version` prints the version, the commit and the build date.

### Run it under systemd

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
4. Read the first-run token: `sudo journalctl -u snagarr`.

## First run

On first start, Snagarr creates the admin user. It issues one token. It prints
both to standard output:

```
  Snagarr is ready. Open the setup URL to finish:

    http://localhost:8080/#token=sngr_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

  Admin token: sngr_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
  Keep it safe — it is not shown again.
```

Read it from the container logs:

```sh
docker compose logs snagarr
```

Or, with plain Docker:

```sh
docker logs snagarr
```

The token is shown once. Snagarr stores only its SHA-256 digest. A lost token
cannot be recovered. Issue a new one from the settings UI.

Open the setup URL. The web client takes the token out of the fragment. It
stores the token. It then removes the token from the URL.

The URL uses `general.public_url` when that setting is set. Otherwise it uses
the listen address, with `localhost` in place of an empty host.

Go to `/setup` to run the four-step wizard: TMDB, media server, Radarr and
Sonarr. Every step can be skipped.

## Port

Snagarr listens on `:8080` by default. Change it with `SNAGARR_ADDR` or with
`--addr`.

The listen address is a Go address string:

| Value | Effect |
|-------|--------|
| `:8080` | Every interface, port 8080. |
| `127.0.0.1:8080` | Loopback only. |
| `0.0.0.0:9090` | Every interface, port 9090. |

Snagarr serves plain HTTP. It has no TLS options. Put a reverse proxy in front
of it for HTTPS.

## Reverse proxy

Forward the whole host to port 8080.

1. Terminate TLS at the proxy.
2. Forward `/` to `http://snagarr:8080`.
3. Set `SNAGARR_PUBLIC_URL` to the external URL, for example
   `https://snagarr.example.com`.

Three rules matter:

- **Use a host, not a subpath.** The web client calls `/api/v1` from the
  domain root. A proxy that mounts Snagarr under `/snagarr/` breaks it.
- **Leave `/api/v1/webhooks/` open.** Radarr, Sonarr, Tautulli and Emby
  authenticate with a query parameter, not a header. Extra proxy authentication
  on that path stops them. See [webhooks.md](webhooks.md).
- **Do not add CORS headers upstream.** Snagarr sends none. Every client that
  needs a browser must run on the Snagarr origin.

Snagarr uses the `X-Forwarded-For` and `X-Real-IP` headers for its request log
only. No behaviour depends on the client address.

### Caddy

```caddy
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

`GET /api/v1/health` needs no token. It answers:

```json
{ "status": "ok", "version": "0.1.0" }
```

```sh
curl http://localhost:8080/api/v1/health
```

The container image is distroless. It holds no shell and no `curl`, so a
`HEALTHCHECK` inside the container is not possible. Check the endpoint from
outside the container.

`GET /api/v1/status` needs a token. It reports the item counts, the last sync
times and which services are configured.

## Backup

Back up `snagarr.db` and `secret.key` together.

> Snagarr cannot read any stored setting without `secret.key`. A database
> restored beside a new key loses every API key and every server token.

The database uses write-ahead logging, so a live copy of `snagarr.db` alone can
be incomplete. Stop the process first.

1. Stop Snagarr: `docker compose stop snagarr`.
2. Copy the whole data directory to your backup target.
3. Start Snagarr: `docker compose start snagarr`.

For a named volume:

```sh
docker compose stop snagarr
docker run --rm -v snagarr:/data -v "$PWD":/backup alpine \
  tar czf /backup/snagarr-backup.tar.gz -C /data .
docker compose start snagarr
```

### Restore

1. Stop Snagarr.
2. Put `snagarr.db` and `secret.key` back in the data directory.
3. Delete any `snagarr.db-wal` and `snagarr.db-shm` left from an older copy.
4. Start Snagarr.

Tokens survive a restore. The indexes rebuild on the first reconcile pass.

## Upgrade

Snagarr applies its database migrations at start-up. No manual step is needed.

Migrations move forward only. **Back up before you upgrade.** A downgrade needs
the old database file.

With Compose:

```sh
docker compose pull
docker compose up -d
```

With plain Docker:

```sh
docker pull ghcr.io/sirrobot01/snagarr:latest
docker stop snagarr
docker rm snagarr
# then run the same `docker run` command again
```

With the binary:

1. Stop the service: `sudo systemctl stop snagarr`.
2. Replace the binary: `sudo install -m 0755 snagarr /usr/local/bin/snagarr`.
3. Start the service: `sudo systemctl start snagarr`.

Check the running version afterwards:

```sh
curl -s http://localhost:8080/api/v1/health
```

## Build from source

You need Go, Node 22 and [Task](https://taskfile.dev).

```sh
task install   # Go tools and web dependencies
task build     # builds the UI, then bin/snagarr
```

`task build` writes the React client to `internal/web/dist`. The Go build then
embeds it. A binary built without that step serves the API but returns 404 for
the UI.

Other tasks:

| Task | What it does |
|------|--------------|
| `task web` | Builds the React client only. |
| `task dev` | Runs the API with live reload and the Vite dev server. |
| `task test` | Runs the Go tests and the web tests. |
| `task lint` | Runs golangci-lint and checks the formatting. |
| `task docker` | Builds the local Docker image from `Dockerfile`. |
| `task snapshot` | Builds an unpublished release with GoReleaser. |
| `task clean` | Removes the build output. |

To build the image without Task:

```sh
docker build -t snagarr:dev .
```

`Dockerfile` builds the client and the binary from source. `Dockerfile.release`
is for GoReleaser only. It copies binaries that GoReleaser has already built.

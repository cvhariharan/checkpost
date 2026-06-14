---
title: Setup
weight: 10
---

# Setup

The repository includes a Docker Compose setup

```sh
curl -fsSL https://raw.githubusercontent.com/cvhariharan/checkpost/refs/heads/master/docker-compose.yml -o docker-compose.yml
docker compose up -d
```

Open `http://localhost:1323` and sign in with `checkpost_admin` / `checkpost_password`. The Compose setup uses `secret-key` as its enrollment key.

## HTTPS

Osquery's [TLS plugin](https://osquery.readthedocs.io/en/stable/deployment/remote/) expects an HTTPS endpoint. Enable Checkpost's built-in TLS or put it behind a TLS-terminating proxy.

There is a commented Caddy service available in `docker-compose.yml`. Point the hostname at the Docker host, set `CHECKPOST_APP__ROOT_URL` to its `https://` URL, then uncomment the Caddy service and volumes. `app.root_url` must match the URL enrolled hosts can reach.

## Configuration

Checkpost reads TOML, then applies environment variables over it:

```sh
cp config.toml.example config.toml
checkpost server --config config.toml
```

Environment variables use `CHECKPOST_<SECTION>__<KEY>`:

```sh
CHECKPOST_APP__ROOT_URL=https://checkpost.example.com
CHECKPOST_DB__HOST=postgres
CHECKPOST_RESULTS__PARQUET__ROOT=/var/lib/checkpost/results
```

See the [configuration reference](configuration.md) for every setting and its default.

## Enroll hosts

Before enrolling a host, confirm that:

- `app.root_url` uses HTTPS and is reachable from the host.
- `app.enrollment_key` is set to a key.
- `osquery_bootstrap.enabled` is `true`.
- The package URL and SHA256 are configured for the target platform.

Open **Inventory**, select **Install osquery**, and choose Linux, macOS, or Windows. Checkpost shows the generated install command and any configuration warnings.

Linux uses the generic osquery tarball and requires systemd and `tar`. macOS uses the configured universal PKG. Windows uses the configured amd64 MSI and requires an Administrator PowerShell session. Each script verifies the package SHA256 before installation. If osquery is already installed, the script keeps the binary and replaces its Checkpost enrollment configuration.

## Build from source

The binary embeds the Svelte frontend. Building requires Go with CGO and Node.js.

```sh
make
./checkpost server
```

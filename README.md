<a href="https://zerodha.tech">
  <img src="https://zerodha.tech/static/images/github-badge.svg" align="right" />
</a>

<br clear="all" />

<div align="center">
    <picture>
        <source srcset="./docs/site/static/checkpost-banner-light.svg" media="(prefers-color-scheme: light)">
        <img src="./docs/site/static/checkpost-banner-dark.svg" width="265">
    </picture>
</div>

<h4 align="center">An open-source <a href="https://osquery.io">osquery</a> manager</h4>

<div align="center">
    <img src="./docs/site/static/screenshots/host-details.png" width="900">
</div>
<br/>

Checkpost is an osquery manager that implements the osquery remote configuration endpoints. It enrolls hosts, serves osquery configuration, schedules queries, evaluates policies, and collects results without needing a separate management stack, all in a single binary.

The system is read-only by design: Checkpost only observes endpoints and doesn't make any changes. Use it to automate posture checks, audit endpoint configurations, investigate hosts with ad-hoc queries, and scan files with YARA.

## Features

- **Inventory**: track hosts, owners, asset IDs, groups, and last-seen state.
- **Scheduled queries**: run queries on a schedule and ship results to multiple backends.
- **Ad-hoc queries**: run on-demand SQL queries against hosts.
- **Policies**: check device posture across enrolled hosts.
- **YARA**: scan files for YARA matches.
- **Alerts**: get notified on policy failures.
- **Access control**: role-based access with SSO (OIDC).
- **GitOps**: manage resources defined in YAML via the `checkpost` CLI.

## Quick start

Requires Docker. Starts Checkpost + Postgres:

```sh
docker compose up
```

Open http://localhost:1323 and log in with the default credentials (`checkpost_admin` / `checkpost_password`). Enrollment key defaults to `secret-key`.

### TLS

The default Compose configuration serves Checkpost over HTTP. Osquery agents require an HTTPS endpoint, so enable TLS before enrolling agents.

Choose one of these options:

- Enable Checkpost's built-in TLS and mount the certificate and private key into the container. Set `CHECKPOST_APP__USE_TLS=true`, configure `CHECKPOST_APP__HTTP_TLS_CERT` and `CHECKPOST_APP__HTTP_TLS_KEY`, and set `CHECKPOST_APP__ROOT_URL` to the HTTPS URL agents will use.
- Put Checkpost behind a TLS-terminating reverse proxy such as Caddy. Set `CHECKPOST_APP__ROOT_URL` to the proxy's public HTTPS URL.

The commented Caddy service in `docker-compose.yml` shows a minimal reverse-proxy setup. Replace `checkpost.example.com` with a domain that resolves to the Docker host, update `CHECKPOST_APP__ROOT_URL` to match, and uncomment the Caddy service and volumes. Caddy will obtain and renew a publicly trusted certificate automatically.

### Configuration

Copy the template and edit as needed (TLS, OIDC/SSO, database, osquery bootstrap packages):

```sh
cp config.toml.example config.toml
```

All settings can also be set via `CHECKPOST_<SECTION>__<KEY>` environment variables (see `docker-compose.yml`).

## Build from source

Requires Go (with a CGO toolchain) and Node.js:

```sh
make      # builds the frontend + checkpost binary
./checkpost      # runs the server
```

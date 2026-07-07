---
title: Configuration
---

# Configuration

Checkpost reads server settings from `config.toml`. Pass a different path with `--config`.

```sh
checkpost server --config /etc/checkpost/config.toml
```

If the file does not exist, Checkpost writes one with default values.

Environment variables load after TOML and take precedence. Uppercase the field name, prefix it with `CHECKPOST_`, and replace each dot with `__`.

```sh
CHECKPOST_APP__ROOT_URL=https://checkpost.example.com
CHECKPOST_DB__PASSWORD=secret
CHECKPOST_RESULTS__PARQUET__ROOT=/var/lib/checkpost/results
```

The defaults below are the runtime defaults. `config.toml.example` uses a few local test paths and placeholders instead.

Durations use Go syntax such as `30m`, `2h`, or `24h`.

## Application

Settings under `[app]` control the HTTP server, built-in administrator, and policy timing.

| Field | Default | What it does |
| --- | --- | --- |
| `app.admin_username` | `checkpost_admin` | Username of the built-in administrator. Checkpost creates this user if needed and keeps its global admin role. |
| `app.admin_password` | `checkpost_password` | Password of the built-in administrator. Checkpost resets it from this value on every start. |
| `app.http_tls_cert` | `server_cert.pem` | Certificate file used when `app.use_tls` is true. |
| `app.http_tls_key` | `server_key.pem` | Private key used when `app.use_tls` is true. |
| `app.root_url` | `http://localhost:1323` | Public URL used for OIDC callbacks, links, and osquery bootstrap scripts. |
| `app.enrollment_key` | Generated | Shared secret accepted when an osquery host enrolls. |
| `app.use_tls` | `false` | Serves HTTPS directly from Checkpost instead of plain HTTP. |
| `app.policy_update_interval` | `1h` | How often hosts receive policy queries in their osquery configuration. |
| `app.policy_stale_after` | `2h` | Age after which a policy result is treated as stale. Also used by offline alert evaluation. |

## OIDC

Settings under `[app.oidc]` enable single sign-on. `issuer`, `client_id`, and `client_secret` must either all be set or all be empty.

| Field | Default | What it does |
| --- | --- | --- |
| `app.oidc.issuer` | Empty | OIDC issuer URL used for provider discovery and token validation. |
| `app.oidc.client_id` | Empty | Client ID registered with the OIDC provider. |
| `app.oidc.client_secret` | Empty | Client secret registered with the OIDC provider. |
| `app.oidc.label` | `Company SSO` | Text shown on the login button. |
| `app.oidc.redirect_url` | Empty | Callback URL sent to the provider. Empty uses `<app.root_url>/auth/callback`. |
| `app.oidc.auth_url` | Empty | Overrides the discovered authorization endpoint. |
| `app.oidc.token_url` | Empty | Overrides the discovered token endpoint. |
| `app.oidc.scopes` | `openid`, `profile`, `email`, `groups` | Scopes requested during login. |
| `app.oidc.groups_claim` | `groups` | ID token claim read for user-group memberships. |
| `app.oidc.allowed_domains` | Empty | Email domains allowed to sign in. Empty allows every domain. |
| `app.oidc.auto_create_users` | `true` | Creates a Checkpost user on the first successful OIDC login. |
| `app.oidc.default_role` | Empty | Global role granted to a newly created OIDC user. Accepts `admin`, `operator`, `analyst`, or `viewer`. Empty grants no role. |

The URL overrides are mainly useful when the browser and server reach the provider through different addresses.

## Sessions

| Field | Default | What it does |
| --- | --- | --- |
| `app.session.ttl` | `8h` | Lifetime of the browser cookie and its PostgreSQL session record. |

## Osquery bootstrap

The `[osquery_bootstrap]` section controls the install commands shown under **Inventory > Install osquery**.

| Field | Default | What it does |
| --- | --- | --- |
| `osquery_bootstrap.enabled` | `true` | Enables generated host installation scripts. |
| `osquery_bootstrap.linux.tarball_amd64.url` | Empty | URL of the Linux amd64 osquery tarball. |
| `osquery_bootstrap.linux.tarball_amd64.sha256` | Empty | SHA256 checksum for the Linux amd64 tarball. |
| `osquery_bootstrap.linux.tarball_arm64.url` | Empty | URL of the Linux arm64 osquery tarball. |
| `osquery_bootstrap.linux.tarball_arm64.sha256` | Empty | SHA256 checksum for the Linux arm64 tarball. |
| `osquery_bootstrap.macos.pkg_universal.url` | Empty | URL of the universal macOS PKG. |
| `osquery_bootstrap.macos.pkg_universal.sha256` | Empty | SHA256 checksum for the macOS PKG. |
| `osquery_bootstrap.windows.msi_amd64.url` | Empty | URL of the Windows amd64 MSI. |
| `osquery_bootstrap.windows.msi_amd64.sha256` | Empty | SHA256 checksum for the Windows MSI. |

## Database

Settings under `[db]` configure PostgreSQL.

| Field | Default | What it does |
| --- | --- | --- |
| `db.dbname` | `checkpost` | PostgreSQL database name. |
| `db.host` | `localhost` | PostgreSQL hostname or IP address. |
| `db.port` | `5432` | PostgreSQL port. |
| `db.user` | `checkpost` | PostgreSQL user. |
| `db.password` | `checkpost` | PostgreSQL password. |

## Results

Checkpost can write scheduled-query rows to more than one backend. The `reader` setting chooses which readable backend powers the web interface.

| Field | Default | What it does |
| --- | --- | --- |
| `results.reader` | Empty | Read backend for the web interface. Accepts `parquet` or `clickhouse`. Empty prefers Parquet, then ClickHouse. |

The selected backend must be enabled. NDJSON is write-only.

### Parquet

| Field | Default | What it does |
| --- | --- | --- |
| `results.parquet.enabled` | `true` | Writes rows to local Parquet files and enables DuckDB reads. |
| `results.parquet.root` | `./data/results` | Root directory for Parquet files. Use an absolute path in production. |
| `results.parquet.duckdb_path` | Empty | Path to a persistent DuckDB catalog. Empty keeps the catalog in memory. |

Parquet is the simplest option for a single Checkpost instance. It is always treated as a required result sink when enabled.

### NDJSON

| Field | Default | What it does |
| --- | --- | --- |
| `results.ndjson.enabled` | `false` | Writes scheduled-query rows as newline-delimited JSON. |
| `results.ndjson.path` | `stdout` | Output file. Empty or `stdout` writes to standard output. Files are opened in append mode. |
| `results.ndjson.required` | `false` | Propagates submission errors instead of treating this backend as best effort. |

Checkpost does not rotate the output file.

### ClickHouse

| Field | Default | What it does |
| --- | --- | --- |
| `results.clickhouse.enabled` | `false` | Writes rows to ClickHouse and makes it available as a reader. |
| `results.clickhouse.dsn` | Empty | ClickHouse connection string. Required when the backend is enabled. |
| `results.clickhouse.table` | `query_results` | Table used for scheduled-query rows. Checkpost creates it on startup. |
| `results.clickhouse.ttl_days` | `0` | Global row retention in days. `0` disables the TTL. |
| `results.clickhouse.required` | `false` | Propagates submission errors instead of treating this backend as best effort. |

#### Creating the table manually

On startup Checkpost probes for the table with `EXISTS TABLE` and issues a `CREATE TABLE IF NOT EXISTS` if it is missing. If the DSN user lacks DDL permission, create the table manually with the statement below before enabling the backend.

```sql
CREATE TABLE IF NOT EXISTS query_results (
    schedule_uuid UUID,
    sql_version   Int32,
    schedule_name String,
    node_id       Int64,
    unix_time     DateTime64(6, 'UTC'),
    calendar_time String,
    action        LowCardinality(String),
    row_hash      String,
    ingested_at   DateTime64(6, 'UTC') DEFAULT now64(6),
    columns       Map(String, String)
) ENGINE = MergeTree
PARTITION BY (schedule_uuid, toYYYYMM(unix_time))
ORDER BY (schedule_uuid, sql_version, node_id, unix_time);
```

Replace `query_results` with the value of `results.clickhouse.table` if you changed it.

To enforce retention yourself, append a `TTL` clause matching `results.clickhouse.ttl_days`:

```sql
) ENGINE = MergeTree
PARTITION BY (schedule_uuid, toYYYYMM(unix_time))
ORDER BY (schedule_uuid, sql_version, node_id, unix_time)
TTL toDateTime(unix_time) + INTERVAL 30 DAY;
```

## Alerts

| Field | Default | What it does |
| --- | --- | --- |
| `alerts.enabled` | `false` | Starts the alert engine and registers webhook and SMTP delivery. |

### SMTP

Used by alert targets

| Field | Default | What it does |
| --- | --- | --- |
| `alerts.smtp.host` | Empty | SMTP server hostname. Empty disables SMTP delivery. |
| `alerts.smtp.port` | `587` | SMTP server port. |
| `alerts.smtp.username` | Empty | Username used for SMTP authentication. |
| `alerts.smtp.password` | Empty | Password used for SMTP authentication. Required when a username is set. |
| `alerts.smtp.from` | `checkpost@example.com` | Sender address used for alert emails. |
| `alerts.smtp.tls` | `starttls` | Connection mode. Accepts `starttls`, `implicit`, or `none`. |

---
title: How it works
weight: 15
---

# How it works

Checkpost runs as a single Go server with the web UI embedded in the binary. It uses PostgreSQL for application state and osquery's TLS plugins for host communication.

Checkpost does not install a separate endpoint agent. Osquery runs on each host, executes SQL locally, and sends results back to Checkpost. The bootstrap scripts enroll the host, store its node key, and write an osquery flagfile that points at Checkpost's config, logger, and distributed-query endpoints.

Schedules are returned through osquery's config endpoint. Policies, live queries, and YARA scans are sent through distributed queries.

## Result storage

Schedules and ad-hoc queries share the same result storage. Result storage is pluggable. New storage backends can be added without changing how schedules or live queries are dispatched.

The current result backends are:

- Parquet with DuckDB, enabled by default, for local storage and browsing.
- ClickHouse for external storage and browsing.
- NDJSON for write-only export to a file or standard output.

## Policies and live queries

Policies are sent through distributed queries when a host is due for a policy check. The due time includes deterministic jitter to spread work across the fleet. A policy passes when the first returned row contains `1`; `0`, no rows, or malformed output fails.

Live queries also use distributed queries. Creating a query run creates one pending machine query per target. When a host polls, Checkpost sends the pending SQL and marks it dispatched. When the result comes back, Checkpost stores completion state in PostgreSQL and writes successful rows to the shared result storage.

YARA scans follow the same dispatch pattern. Checkpost builds osquery `yara_file` SQL from the requested paths and rule URLs, then records matches in PostgreSQL. It does not quarantine, delete, or modify files on the host.

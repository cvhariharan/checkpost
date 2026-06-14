---
title: GitOps
weight: 30
---

# GitOps

`checkpost apply` reconciles policies, schedules, alert targets, alert rules, and YARA signature sources from YAML. The command is idempotent and suitable for CI.

Create an API token under **Settings > API Tokens**, then configure the CLI:

```yaml
# ~/.config/checkpost/config.yaml
server: https://checkpost.example.com
token: cp_example
```

```sh
chmod 0600 ~/.config/checkpost/config.yaml
checkpost apply -f resources/ --recursive --dry-run
checkpost apply -f resources/ --recursive
```

Flags take precedence over `CHECKPOST_SERVER` and `CHECKPOST_TOKEN`, which take precedence over the config file. Directories are not recursive unless `--recursive` is set. `-f -` reads YAML from standard input.

Every resource has a stable `id` handle. Checkpost stores server UUIDs in `checkpost.state.json`; commit this file beside the YAML. `--prune` deletes state-tracked resources absent from the current input, so use it only when applying the complete desired set.

The CLI refuses to send a token over plain HTTP to a non-loopback host. `--insecure` disables that check and TLS certificate verification.

## Resources

One file may contain multiple documents separated by `---`.

### Policy and schedule

```yaml
kind: policy
id: disk-encryption
name: Disk encryption enabled
query: SELECT 1 FROM disk_encryption WHERE encrypted = 1;
platform: darwin
resolution: Enable FileVault.
enabled: true
groups:
  - laptops
---
kind: schedule
id: installed-packages
name: Installed packages
query: SELECT name, version FROM programs;
interval: 3600
snapshot: true
shard: 100
groups:
  - workstations
```

Platforms may be `darwin`, `linux`, `posix`, `windows`, `any`, or `all`. Group names must already exist. Schedule intervals range from 1 to 604800 seconds; shards range from 0 to 100.

### Alert target and rule

```yaml
kind: alert_target
id: security-webhook
name: security-webhook
type: webhook
enabled: true
config:
  url: https://alerts.example.com/checkpost
---
kind: alert_rule
id: offline-laptops
name: offline-laptops
source: machine_offline
severity: medium
enabled: true
evaluation_interval: 300
for: 900
repeat_interval: 86400
params:
  threshold: 24h
targets:
  - security-webhook
```

Rules reference targets by name. `evaluation_interval` must be at least 60 seconds. SMTP targets use `config.recipients` with addresses, `owner`, or `user-group:<name>`.

### YARA source

```yaml
kind: yara_source
id: shared-rules
name: Shared rules
url: https://rules.example.com/base.yar
group: laptops
enabled: true
```

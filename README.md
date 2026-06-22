# MDADM Notifier

**Know when your Linux RAID array is sick — before it's too late.**

A lightweight Docker service that watches `mdadm` software RAID and member-disk SMART health, alerts you when something changes, and serves a built-in dashboard. One container, no agents on every disk, no external monitoring stack required.

[![Docker image](https://img.shields.io/badge/ghcr.io-ardean%2Fmdadm--notifier-2496ED?logo=docker&logoColor=white)](https://github.com/ardean/mdadm-notifier/pkgs/container/mdadm-notifier)
[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](https://go.dev/)

---

## Why use this?

`mdadm` and `smartctl` already know when a disk is failing or an array is degraded — but they only help if someone is looking. Most homelab and small-server setups don't have a dedicated monitoring stack, and RAID status in `/proc/mdstat` is easy to miss until a rebuild is already underway.

MDADM Notifier closes that gap:

- **Proactive checks** — periodically runs `mdadm -D` and `smartctl` on every member disk
- **Delta alerts** — tracks SMART counters between runs and notifies when error counts *increase*, not just when they cross a threshold
- **Automatic self-tests** — schedules short and long SMART self-tests so marginal drives surface problems early
- **Actionable notifications** — Discord, generic webhooks (ntfy, Gotify, custom relays), or local logs, each prefixed with the host name
- **Live dashboard** — dark-mode web UI with RAID status, rebuild ETA, per-disk SMART data, and a JSON API

Runs as a single privileged container with a few volume mounts. SMART state persists under `/data` across restarts.

---

## Quick start

```yml
services:
  mdadm-notifier:
    image: ghcr.io/ardean/mdadm-notifier:master
    volumes:
      - /dev/md0:/dev/md0          # your RAID device
      - /etc/hostname:/etc/hostname:ro
      - mdadm-notifier-data:/data
    environment:
      - NOTIFY_METHODS=discord
      - DISCORD_TOKEN=your-bot-token
      - DISCORD_CHANNEL_ID=your-channel-id
    ports:
      - "8080:8080"
    privileged: true
    restart: unless-stopped

volumes:
  mdadm-notifier-data:
```

Open `http://<host>:8080` for the dashboard. See [Docker Compose](#docker-compose) below for a fuller example and [Configuration](#configuration) for every option.

---

## Features

### RAID monitoring

- Failed devices and non-clean array states (`degraded`, `recovering`, etc.)
- Rebuild / resync / recovery progress with ETA and speed from `/proc/mdstat`
- Live progress updates over WebSocket during active sync (no extra SMART load)

### SMART health

- Overall health, marginal attributes, and threshold failures
- Critical counters: reallocated sectors, uncorrectable errors, pending/offline sectors, device error log count
- Configurable thresholds per counter type
- **Delta tracking** — persisted history under `SMART_STATE_DIR` alerts on counter *increases* between checks

### SMART self-tests

- Automatic short and long self-tests on member disks, scheduled by power-on hours
- Self-test results included in health evaluation
- Configurable intervals and minimum gap between tests on the same disk

### Notifications

| Channel | Use case |
|---------|----------|
| **Discord** | Bot posts to a channel (default) |
| **Webhook** | ntfy, Gotify, Home Assistant, custom HTTP relay |
| **Log** | Local stdout for debugging or log shipping |

Every message is prefixed with the server hostname (`[my-nas] …`). Alerts fire on first detection, on issue changes, when all issues clear, and optionally on reminders for ongoing problems.

### Web dashboard

Built-in UI (no separate frontend to deploy):

- Overall health badge and last-check timestamp
- RAID detail, member disks, sync progress
- Per-disk SMART attributes, monitored counters (with previous values), self-test history
- Manual refresh button; auto-refresh every 30 s; 1 s WebSocket updates during rebuilds
- JSON status API at `/api/status`

Set `WEB_ENABLED=false` to disable.

---

## How it works

```
┌─────────────────────────────────────────────────────────┐
│                   mdadm-notifier                        │
│                                                         │
│  ┌──────────┐   ┌──────────┐   ┌─────────────────────┐  │
│  │  mdadm   │   │ smartctl │   │  SMART state (/data)│  │
│  │  -D      │   │  -x,     │   │  (delta tracking)   │  │
│  │  mdstat  │   │  selftest│   └─────────────────────┘  │
│  └────┬─────┘   └────┬─────┘                            │
│       └──────┬───────┘                                  │
│              ▼                                          │
│       health evaluation                                 │
│              │                                          │
│     ┌────────┼────────┐                                 │
│     ▼        ▼        ▼                                 │
│  Discord  webhook   log                                 │
│                                                         │
│  Web dashboard (:8080)  ←  WebSocket during sync        │
└─────────────────────────────────────────────────────────┘
```

On each check interval the watcher inspects the configured array, evaluates every member disk, compares SMART counters to persisted state, runs due self-tests, and sends notifications only when something is wrong or has changed.

---

## Docker Compose

```yml
services:
  mdadm-notifier:
    container_name: mdadm-notifier
    image: ghcr.io/ardean/mdadm-notifier:master
    volumes:
      - /dev/md/data:/dev/md0
      - /etc/hostname:/etc/hostname:ro
      - mdadm-notifier-data:/data
    environment:
      - NOTIFY_METHODS=log,discord
      - DISCORD_TOKEN=your-bot-token
      - DISCORD_CHANNEL_ID=your-channel-id
      - SMART_STATE_DIR=/data
      - WEB_ENABLED=true
      - WEB_PORT=8080
    ports:
      - "8080:8080"
    privileged: true
    restart: always

volumes:
  mdadm-notifier-data:
```

**Privileged mode** is required so the container can access block devices for `mdadm` and `smartctl`, and read the host's `/proc/mdstat` for rebuild progress. A `/proc` bind mount is not needed and does not work reliably in Compose.

**Device mapping** — map your RAID device to the path expected by `MD_DEVICE` (default `/dev/md0`). For example, `/dev/md127:/dev/md0`. The kernel name in `/proc/mdstat` (e.g. `md127`) may differ from `MD_DEVICE`; the watcher matches arrays by member disks when names don't line up.

**Hostname** — mount `/etc/hostname` so notifications use the host name instead of the container ID. Alternatively set `SERVER_HOSTNAME` or use the `hostname:` compose field.

**Persistence** — mount `/data` (or a named volume) so SMART counter history survives restarts and delta alerts keep working.

---

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NOTIFY_METHODS` | no | `discord` | Comma-separated: `discord`, `webhook`, `log` |
| `DISCORD_TOKEN` | if `discord` enabled | — | Discord bot token |
| `DISCORD_CHANNEL_ID` | if `discord` enabled | — | Discord channel to post messages to |
| `WEBHOOK_URL` | if `webhook` enabled | — | HTTP endpoint that receives `{"message":"..."}` JSON |
| `MD_DEVICE` | no | `/dev/md0` | RAID array device to monitor |
| `CHECK_INTERVAL` | no | `1h` | How often to check RAID and disk health (e.g. `30m`, `2h`) |
| `SELFTEST_ENABLED` | no | `true` | Enable automatic SMART self-tests and include results in health checks |
| `SELFTEST_CHECK_INTERVAL` | no | `1h` | How often to check whether self-tests are due |
| `SELFTEST_SHORT_INTERVAL` | no | `168h` | Minimum time between short self-tests per disk (`0` disables) |
| `SELFTEST_LONG_INTERVAL` | no | `720h` | Minimum time between long self-tests per disk (`0` disables) |
| `SELFTEST_MIN_GAP` | no | `24h` | Minimum time between any self-tests on the same disk (`0` disables) |
| `SMART_STATE_DIR` | no | `/data` | Directory for persisted SMART counter state (delta tracking) |
| `SMART_REALLOCATED_THRESHOLD` | no | `1` | Alert when reallocated sector count is at or above this value |
| `SMART_UNCORRECTABLE_THRESHOLD` | no | `1` | Alert when reported uncorrectable count is at or above this value |
| `SMART_PENDING_THRESHOLD` | no | `1` | Alert when current pending sector count is at or above this value |
| `SMART_OFFLINE_THRESHOLD` | no | `1` | Alert when offline uncorrectable count is at or above this value |
| `SMART_ERROR_LOG_THRESHOLD` | no | `1` | Alert when device error log count is at or above this value |
| `NOTIFY_STARTUP_SHUTDOWN` | no | `true` | Post notifications when the watcher starts and stops |
| `NOTIFY_REMINDER_INTERVAL` | no | `0` | Re-notify for unchanged ongoing issues after this interval (`0` disables) |
| `SERVER_HOSTNAME` | no | — | Override hostname shown in messages |
| `WEB_ENABLED` | no | `true` | Serve the health dashboard and JSON status API |
| `WEB_PORT` | no | `8080` | TCP port for the dashboard (used when `WEB_ADDR` is unset) |
| `WEB_ADDR` | no | `:8080` | Listen address for the dashboard (overrides `WEB_PORT`) |

Hostname resolution order: `SERVER_HOSTNAME` → `/etc/hostname` → system hostname.

### Notification methods

```yml
# Discord only (default)
NOTIFY_METHODS=discord

# Discord and local logs
NOTIFY_METHODS=discord,log

# Webhook only (ntfy, Gotify, custom relay, etc.)
NOTIFY_METHODS=webhook
WEBHOOK_URL=https://example.com/hook
```

The webhook notifier POSTs JSON:

```json
{"message": "[my-nas] Health check found issues: ..."}
```

---

## Notifications

All messages are prefixed with the hostname:

```
[my-nas] Watcher started — monitoring /dev/md0 every 1h via discord; self-tests every 1h (short 7d, long 30d)
```

| Event | Notification |
|-------|--------------|
| Watcher starts | yes (unless `NOTIFY_STARTUP_SHUTDOWN=false`) |
| Watcher stops | yes (unless `NOTIFY_STARTUP_SHUTDOWN=false`) |
| RAID or disk issue first detected | yes |
| SMART counter increase or other issue change | yes |
| All issues cleared | yes |
| Unchanged ongoing issue on periodic check | no |
| Reminder for unchanged ongoing issue | yes (only if `NOTIFY_REMINDER_INTERVAL` is set) |
| Self-test start failure | yes |
| Healthy periodic check | no (logged locally only) |

Disks are flagged unhealthy when overall SMART health fails, critical attribute counts exceed thresholds, SMART reports marginal attributes or threshold failures, or monitored counters increase since the last check. Example alert excerpt:

```
/dev/sdd: SMART overall-health: PASSED
SMART marginal attributes reported by drive
Reallocated_Sector_Ct: 104
Reported_Uncorrect: 17
Device error log count: 17
Airflow_Temperature_Cel: threshold Past
```

Self-tests use drive power-on hours to decide when the next short or long test is due. Long tests take priority over short tests when both are due on the same check. Only one test runs on a disk at a time, and a configurable minimum gap applies between any two tests on the same disk.

---

## Local development

```bash
# create .env with NOTIFY_METHODS and backend credentials
# optional: SMART_STATE_DIR=./data for local counter persistence
go run .
```

Requires `mdadm` and `smartmontools` installed on the host.

## Build

```bash
docker build -t mdadm-notifier .
```

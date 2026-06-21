# MDADM Notifier

A lightweight Docker service that monitors Linux software RAID arrays and disk SMART health, sending alerts when issues are detected.

The watcher periodically runs `mdadm -D` on the configured array, `smartctl -x` on each member disk, and `smartctl -l xselftest,selftest` for self-test logs. Notifications are prefixed with the server hostname so you can tell which machine reported the issue.

## Features

- Periodic RAID health checks (failed devices, degraded array state)
- SMART health checks on all member disks parsed from the array
- Critical SMART attribute monitoring (reallocated sectors, uncorrectable errors, error log counts, threshold failures)
- Delta tracking for SMART counters between checks (persisted under `/data`)
- Automatic short and long SMART self-tests on member disks
- Self-test results included in disk health evaluation
- Multiple notification methods: Discord, webhook, and log
- Built-in web dashboard for RAID and disk health at a glance
- Alerts when RAID or disk health issues are found or change
- Optional reminders for ongoing unchanged issues
- Hostname included in every message

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

The container needs `privileged: true` so it can access block devices for `mdadm` and `smartctl`.

Map your RAID device to the path expected by `MD_DEVICE` (defaults to `/dev/md0`). Adjust the left-hand side to match your setup, for example `/dev/md127:/dev/md0`.

Mounting `/etc/hostname` lets notifications use the host's name instead of the container ID. You can also set `SERVER_HOSTNAME` or use the `hostname:` compose field instead.

Mount `/data` (or a named volume at `/data`) so SMART counter history survives container restarts. This enables delta alerts when error counts increase between checks.

Open `http://<host>:8080` to view the web dashboard. It shows RAID status, member disk SMART data, monitored counters, self-test history, and full `mdadm` detail. The page refreshes every 30 seconds. Set `WEB_ENABLED=false` to disable it.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NOTIFY_METHODS` | no | `discord` | Comma-separated notification methods: `discord`, `webhook`, `log` |
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
| `NOTIFY_REMINDER_INTERVAL` | no | `0` | Re-notify for unchanged ongoing issues after this interval (`0` disables reminders) |
| `SERVER_HOSTNAME` | no | — | Override hostname shown in messages |
| `WEB_ENABLED` | no | `true` | Serve the health dashboard and JSON status API |
| `WEB_PORT` | no | `8080` | TCP port for the dashboard (used when `WEB_ADDR` is unset) |
| `WEB_ADDR` | no | `:8080` | Listen address for the dashboard (overrides `WEB_PORT`) |

Hostname resolution order: `SERVER_HOSTNAME` → `/etc/hostname` → system hostname.

### Notification methods

Enable one or more methods with `NOTIFY_METHODS`:

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

Disks are flagged unhealthy when overall SMART health fails, when critical attribute counts exceed configured thresholds, when SMART reports marginal attributes or threshold failures, or when monitored counters increase since the last check. Example alert excerpt:

```
/dev/sdd: SMART overall-health: PASSED
SMART marginal attributes reported by drive
Reallocated_Sector_Ct: 104
Reported_Uncorrect: 17
Device error log count: 17
Airflow_Temperature_Cel: threshold Past (46 C)
```

Self-tests use drive power-on hours to decide when the next short or long test is due. Long tests take priority over short tests when both are due on the same check. Only one test runs on a disk at a time, and a configurable minimum gap applies between any two tests on the same disk.

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

# MDADM Notifier

A lightweight Docker service that monitors Linux software RAID arrays and disk SMART health, sending alerts to Discord when issues are detected.

The watcher periodically runs `mdadm -D` on the configured array and `smartctl -a` on each member disk. Notifications are prefixed with the server hostname so you can tell which machine reported the issue.

## Features

- Periodic RAID health checks (failed devices, degraded array state)
- SMART health checks on all member disks parsed from the array
- Automatic short and long SMART self-tests on member disks
- Self-test results included in disk health evaluation
- Discord notifications on startup and shutdown
- Discord alerts when RAID or disk health issues are found
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
    environment:
      - DISCORD_TOKEN=your-bot-token
      - DISCORD_CHANNEL_ID=your-channel-id
    privileged: true
    restart: always
```

The container needs `privileged: true` so it can access block devices for `mdadm` and `smartctl`.

Map your RAID device to the path expected by `MD_DEVICE` (defaults to `/dev/md0`). Adjust the left-hand side to match your setup, for example `/dev/md127:/dev/md0`.

Mounting `/etc/hostname` lets notifications use the host's name instead of the container ID. You can also set `SERVER_HOSTNAME` or use the `hostname:` compose field instead.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DISCORD_TOKEN` | yes | — | Discord bot token |
| `DISCORD_CHANNEL_ID` | yes | — | Discord channel to post messages to |
| `MD_DEVICE` | no | `/dev/md0` | RAID array device to monitor |
| `CHECK_INTERVAL` | no | `1h` | How often to check RAID and disk health (e.g. `30m`, `2h`) |
| `SELFTEST_ENABLED` | no | `true` | Enable automatic SMART self-tests and include results in health checks |
| `SELFTEST_CHECK_INTERVAL` | no | `1h` | How often to check whether self-tests are due |
| `SELFTEST_SHORT_INTERVAL` | no | `168h` | Minimum time between short self-tests per disk (`0` disables) |
| `SELFTEST_LONG_INTERVAL` | no | `720h` | Minimum time between long self-tests per disk (`0` disables) |
| `SELFTEST_MIN_GAP` | no | `24h` | Minimum time between any self-tests on the same disk (`0` disables) |
| `NOTIFY_STARTUP_SHUTDOWN` | no | `true` | Post Discord messages when the watcher starts and stops |
| `SERVER_HOSTNAME` | no | — | Override hostname shown in messages |

Hostname resolution order: `SERVER_HOSTNAME` → `/etc/hostname` → system hostname.

## Notifications

All messages are prefixed with the hostname:

```
[my-nas] Watcher started — monitoring /dev/md0 every 1h; self-tests every 1h (short 7d, long 30d)
```

| Event | Discord notification |
|-------|---------------------|
| Watcher starts | yes (unless `NOTIFY_STARTUP_SHUTDOWN=false`) |
| Watcher stops | yes (unless `NOTIFY_STARTUP_SHUTDOWN=false`) |
| RAID or disk issue found | yes |
| Self-test start failure | yes |
| Healthy periodic check | no (logged locally only) |

Self-tests use drive power-on hours to decide when the next short or long test is due. Long tests take priority over short tests when both are due on the same check. Only one test runs on a disk at a time, and a configurable minimum gap applies between any two tests on the same disk.

## Local development

```bash
# create .env with DISCORD_TOKEN and DISCORD_CHANNEL_ID
go run .
```

Requires `mdadm` and `smartmontools` installed on the host.

## Build

```bash
docker build -t mdadm-notifier .
```

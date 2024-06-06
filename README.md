# MDADM Notifier
### Docker Image to send Software-RAID Events to Discord

Example Docker Compose file:

```yml
services:
  mdadm-notifier:
    container_name: mdadm-notifier
    image: ghcr.io/ardean/mdadm-notifier:master
    volumes:
      - /dev/md/data:/dev/md0
    environment:
      - DISCORD_TOKEN=EDkwNzEzNDI3MzE0NjEwMTc3.ZUzzhQ.eIeMFlRV8KjQi784E-XXXXXX
      - DISCORD_CHANNEL_ID=8007183393314XXXXX
    privileged: true
    restart: always
```

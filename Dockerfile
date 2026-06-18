FROM golang:1.22-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o notify .

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y mdadm ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/notify ./notify

CMD ["mdadm", "--monitor", "--mail", "", "--program", "./notify", "/dev/md0"]

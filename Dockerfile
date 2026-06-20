FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o mdadm-notifier .

FROM alpine:3.21

RUN apk add --no-cache mdadm smartmontools ca-certificates && \
    mkdir -p /data

WORKDIR /app

VOLUME /data

COPY --from=builder /app/mdadm-notifier ./mdadm-notifier

CMD ["./mdadm-notifier"]

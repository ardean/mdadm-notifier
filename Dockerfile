FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o notify .

FROM alpine:3.21

RUN apk add --no-cache mdadm smartmontools ca-certificates

WORKDIR /app

COPY --from=builder /app/notify ./notify

CMD ["./notify"]

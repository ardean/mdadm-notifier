FROM rust:1.73.0 as builder

RUN mkdir /app
WORKDIR /app

COPY . .

RUN cargo build --release

FROM ubuntu:22.04

RUN apt-get update -y
RUN apt-get upgrade -y
RUN apt-get install -y mdadm ca-certificates

RUN mkdir /app
WORKDIR /app

COPY --from=builder /app/target/release/mdadm-notifier ./notify
RUN chmod +x ./notify

CMD mdadm --monitor --mail "" --program ./notify --test /dev/md0
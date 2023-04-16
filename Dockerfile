FROM golang:1.20.1-buster as builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./

RUN go build -o server -ldflags="-s -w" main.go

################################################################

FROM debian:buster-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server /app/server

VOLUME [ "/app/config" ]

CMD [ "/app/server" ]
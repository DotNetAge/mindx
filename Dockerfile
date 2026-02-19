FROM golang:1.25-bullseye AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
    nodejs \
    npm \
    zip \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make build

FROM debian:bullseye-slim

WORKDIR /mindx

COPY --from=builder /app/releases /releases
COPY --from=builder /app/dist /mindx
COPY --from=builder /app/bin /mindx/bin

CMD ["/mindx/bin/mindx", "help"]

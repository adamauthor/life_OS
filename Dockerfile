FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/life-os-bot ./cmd/bot
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/life-os-migrate ./cmd/migrate

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/life-os-bot /app/life-os-bot
COPY --from=builder /out/life-os-migrate /app/life-os-migrate
COPY migrations /app/migrations

ENV MIGRATIONS_SOURCE=file:///app/migrations

CMD ["./life-os-bot"]

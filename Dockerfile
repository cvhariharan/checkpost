FROM node:26-bookworm AS web-builder

WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-trixie AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/checkpost ./cmd/checkpost

FROM debian:trixie-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/checkpost /usr/local/bin/checkpost
COPY config.toml.example /etc/checkpost/config.toml

EXPOSE 1323

ENTRYPOINT ["checkpost"]
CMD ["server", "--config", "/etc/checkpost/config.toml"]

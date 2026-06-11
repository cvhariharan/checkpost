ARG TARGETARCH

FROM --platform=$BUILDPLATFORM node:26-bookworm AS web-builder

WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS go-builder-base

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM go-builder-base AS go-builder-amd64
ENV CC=gcc CXX=g++

FROM go-builder-base AS go-builder-arm64
RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu g++-aarch64-linux-gnu \
    && rm -rf /var/lib/apt/lists/*
ENV CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++

FROM go-builder-${TARGETARCH} AS go-builder

ARG TARGETARCH

COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux GOARCH="$TARGETARCH" go build -o /out/checkpost ./cmd/checkpost

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

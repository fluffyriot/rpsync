# Stage 1: Builder
FROM --platform=$BUILDPLATFORM golang:1.26.5 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

RUN apt-get update && apt-get install -y git bash curl ca-certificates && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o rpsync .

# Stage 2: Runtime
FROM debian:trixie

WORKDIR /app

RUN echo "deb http://security.debian.org/debian-security trixie-security main" >> /etc/apt/sources.list.d/debian-security.list \
 && apt-get update \
 && apt-get install -y bash netcat-openbsd ca-certificates chromium fonts-liberation util-linux \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /app/outputs \
 && mkdir -p /app/certs

COPY --from=builder /app/rpsync .

COPY templates/ templates/
COPY sql/schema/ sql/schema/
COPY static/ static/
COPY docker/entrypoint.sh .

RUN chmod +x entrypoint.sh

ENV PATH="/usr/local/bin:${PATH}"

RUN groupadd -r appuser && useradd -r -g appuser -u 1000 -m -d /home/appuser appuser

RUN chown -R appuser:appuser /app /home/appuser

USER appuser

ENTRYPOINT ["./entrypoint.sh"]

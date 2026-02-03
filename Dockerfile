# Stage 1: Builder
FROM --platform=$BUILDPLATFORM golang:1.25.6 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

RUN apt-get update && apt-get install -y git bash curl ca-certificates && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o rpsync .

# Stage 2: Runtime
FROM --platform=$TARGETPLATFORM debian:bookworm

WORKDIR /app

RUN apt-get update \
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

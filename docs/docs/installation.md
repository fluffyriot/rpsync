---
layout: default
title: Installation
---

# Installation

## Prerequisites

* On Windows, [install WSL](https://learn.microsoft.com/en-us/windows/wsl/install) first.
* Docker
* Docker Compose

## Steps

### 0. Verify Docker Installation

Run the following commands to verify Docker is installed correctly:

```bash
docker --version
docker compose version
```

### 1. Create a Working Directory

```bash
mkdir rpsync
cd rpsync
```

> **IMPORTANT**
> If you plan to sync Instagram data, complete the [Instagram Setup](./docs/setup/instagram) section first.

---

## Environment Configuration

Create a `.env` file in the newly created directory using the template below:

```env
POSTGRES_DB=rpsync-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=password123
POSTGRES_PORT=5435
POSTGRES_HOST=db

APP_PORT=22347
HTTP_PORT=8081
HTTPS_PORT=8443

INSTAGRAM_API_VERSION=24.0
LOCAL_IP=XXX.XXX.XXX.XXX
TOKEN_ENCRYPTION_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OAUTH_ENCRYPTION_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

FACEBOOK_APP_ID=xxxxxxxxxxxxxxxx
FACEBOOK_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Environment Variable Reference

* **POSTGRES_***
  Database configuration. Change `POSTGRES_PASSWORD` to a secure value. Other values can remain unchanged.

* **POSTGRES_PORT**
  External port used by PostgreSQL in Docker.

* **APP_PORT**
  Internal HTTP port used by the application container.

* **HTTP_PORT / HTTPS_PORT**
  Ports exposed by Caddy. Ensure these do not conflict with other services on your machine.

* **INSTAGRAM_API_VERSION**
  Facebook Graph API version. Version `24.0` is tested; other versions may cause issues.

* **LOCAL_IP**
  The local IP address used to access the application.

* **TOKEN_ENCRYPTION_KEY**
  Used to encrypt tokens stored in the database.

* **OAUTH_ENCRYPTION_KEY**
  Used to encrypt OAuth URLs during Facebook login.

  Generate both encryption keys with:

  ```bash
  openssl rand -base64 32
  ```

* **FACEBOOK_APP_ID / FACEBOOK_APP_SECRET**
  Required for Instagram synchronization. See [Instagram Setup](./docs/setup/instagram).

---

## Docker Setup

Create a `docker-compose.yml` file in the previously created directory. No changes are required.

```yaml
version: "3.9"

services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - db_data:/var/lib/postgresql/data
    ports:
      - "${POSTGRES_PORT}:5432"

  app:
    image: fluffyriot/rpsync:latest
    restart: unless-stopped
    env_file: .env
    depends_on:
      - db
    ports:
      - "${APP_PORT}:${APP_PORT}"
    volumes:
      - ./outputs:/app/outputs

  caddy:
    image: caddy:latest
    restart: unless-stopped
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - ./certs:/certs
    ports:
      - "${HTTPS_PORT}:${HTTPS_PORT}"
      - "${HTTP_PORT}:80"
    depends_on:
      - app

volumes:
  db_data:
```

---

## HTTPS and Certificates

### 1. Create a Caddyfile Template

Create `Caddyfile.template`:

```caddyfile
# Redirect HTTP to HTTPS
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}

# HTTPS site with self-signed certificate
:${HTTPS_PORT} {
    tls /certs/server.crt /certs/server.key
    reverse_proxy app:${APP_PORT}
}
```

### 2. Generate the Caddyfile

```bash
export $(grep -v '^#' .env | xargs)
envsubst < Caddyfile.template > Caddyfile
```

### 3. Generate a Self-Signed Certificate

```bash
IP=${LOCAL_IP:-127.0.0.1}
mkdir -p certs
sudo openssl req -x509 -nodes -days 365 \
  -newkey rsa:2048 \
  -keyout certs/server.key \
  -out certs/server.crt \
  -subj "/CN=${IP}" \
  -addext "subjectAltName=IP:${IP}"

echo "Self-signed certificate generated for ${IP}"
```

---

## Running the Application

1. Pull the required Docker images:

   ```bash
   docker compose pull
   ```

2. Start the stack:

   ```bash
   docker compose up -d
   ```

3. Open your browser and navigate to:

   ```
   https://LOCAL_IP:HTTPS_PORT
   ```

   Replace `LOCAL_IP` and `HTTPS_PORT` with the values from your `.env` file.

---

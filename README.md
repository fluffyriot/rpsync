# Commission Tracker

Collect and track your online presence statistics in a local database that runs entirely on your machine.

This project is designed for digital creators who want insight into their social media performance without relying on expensive or data‑intrusive third‑party tools.

---

## Table of Contents

* [Quick Start](#quick-start)
* [Environment Configuration](#environment-configuration)
* [Docker Setup](#docker-setup)
* [HTTPS and Certificates](#https-and-certificates)
* [Running the Application](#running-the-application)
* [Usage](#usage)
* [Instagram Sync Setup](#setup-for-instagram-sync)
* [Motivation](#motivation)
* [Contributing](#contributing)

---

## Quick Start

### Prerequisites

* For Windows, [install WSL](https://learn.microsoft.com/en-us/windows/wsl/install) first.
* Docker
* Docker Compose

Verify your installation:

```bash
docker --version
docker compose version
```

### 1. Create a Working Directory

```bash
mkdir commission-tracker
cd commission-tracker
```

> **IMPORTANT**
> If you plan to sync Instagram data, complete the [Instagram Sync Setup](#setup-for-instagram-sync) section first.

---

## Environment Configuration

Create a `.env` file in the project root using the template below:

```env
BASE_URL=https://${LOCAL_IP}:${HTTPS_PORT}

POSTGRES_DB=commission-tracker-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=password123
POSTGRES_PORT=5435
POSTGRES_HOST=db

PORT=8080
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
  Required for Instagram synchronization. See [Instagram Sync Setup](#setup-for-instagram-sync).

---

## Docker Setup

Create a `docker-compose.yml` file in the project root. No changes are required.

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
    image: fluffyriot/commission-tracker:latest
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
      - "${HTTPS_PORT}:443"
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
${LOCAL_IP}:${HTTPS_PORT} {
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

## Usage

All features are available through the web UI.

Instagram data synchronization requires additional setup via Meta (Facebook) Developer tools.

---

## Setup for Instagram Sync

> **Important**
> Your Instagram account must:
>
> * Be a **Business** or **Creator** account
> * Be **connected to a Facebook Page**
>
> Reference:
>
> * [https://help.instagram.com/1980623166138346/](https://help.instagram.com/1980623166138346/)
> * [https://www.facebook.com/help/instagram/790156881117411](https://www.facebook.com/help/instagram/790156881117411)

### Steps

1. Go to **Meta for Developers** and create an account:
   [https://developers.facebook.com/apps/](https://developers.facebook.com/apps/)

2. Create a new app.

   * Select:

     * *Manage Messaging and content on Instagram*
     * *Manage everything on your Page*

3. In the App Dashboard, navigate to **Settings → Basic**.

4. Copy the **App ID** and **App Secret** into your `.env` file.

5. Open **Facebook Login for Business**.

   * Under **Client OAuth Settings**, add the following to **Valid OAuth Redirect URIs**:

     ```
     https://LOCAL_IP:HTTPS_PORT/auth/facebook/callback
     ```

6. Go to **Use Cases**, edit the **Instagram** use case.

7. Under **API setup with Facebook Login**, add permissions in the **Manage content** section.

8. In **Permissions and Features**, enable:

   * Manage insights
   * Manage comments

9. Open **Graph Explorer**:
   [https://developers.facebook.com/tools/explorer/](https://developers.facebook.com/tools/explorer/)

10. Click **Generate Token**, select your Page and Instagram account.

11. Copy and save the numeric Instagram Page ID displayed.

---

## Motivation

This application was created to provide creators with a simple, privacy‑respecting way to analyze their social media presence.

The current release is **alpha** and focuses on manual workflows. Support for additional platforms and automation is planned.

---

## Contributing

Contributions are welcome.

* Submit feature requests or bugs via GitHub Issues
* Open pull requests for improvements or new features

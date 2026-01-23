# RPSync

<img width="1280" height="640" alt="rpsync_banner" src="https://github.com/user-attachments/assets/8e35a1b0-e4b7-4a4d-8644-f65e430635ce" />

---

Collect and track your online presence statistics in a local database that runs entirely on your machine.

**RPSync** is your personal, privacy-first command center for social media analytics. Stop relying on fragmented, invasive, and expensive third-party tools. Run this application locally to collect, visualize, and own your data forever.

**Key Benefits:**
*   **100% Local & Private**: Your data never leaves your machine unless you say so.
*   **Unified Analytics**: View statistics from Instagram, TikTok, Bluesky, Mastodon, and more in one beautiful dashboard.
*   **Data Ownership**: Seamlessly export your data to NocoDB and CSV for external analysis.
*   **No Subscriptions**: Free and open-source.

This app is intended to run in Docker and has both x64 and arm builds. It is intended to run on Debian-like systems and has been tested on Windows using WSL2 Ubuntu and Raspberry Pi running on Raspbian.

## Features

### Data Collection (Sources)
*   **Instagram**: Syncs public creator/business pages (requires Facebook Page connection).
*   **TikTok**: Advanced scraping via TikTok Creator Studio.
*   **Telegram**: Tracks your public channels
*   **Other Platforms**: Bluesky, Mastodon, Murrtube.net, BadPups.com, Google Analytics

### Data Management (Targets)
*   **NocoDB Integration**: Automatically syncs your posts and sources to NocoDB for Airtable-like management.
*   **CSV Exports**: Easy variable data export for offline analysis.

### Important Notes
*   **Instagram Sync**: Requires a Facebook App and a Facebook Page connected to the Instagram account. [Setup Guide](#setup-for-instagram-sync)
*   **TikTok Sync**: Uses TikTok Creator Studio page. Ensure you have access to it.
*   **Telegram Sync**: Requires App creation and Bot setup. [Setup Guide](#setup-for-telegram-sync)

---

## Table of Contents

* [Start Guide](#start-guide)
* [Environment Configuration](#environment-configuration)
* [Docker Setup](#docker-setup)
* [HTTPS and Certificates](#https-and-certificates)
* [Running the Application](#running-the-application)
* [Usage](#usage)
* [Instagram Sync Setup](#setup-for-instagram-sync)
* [Telegram Sync Setup](#setup-for-telegram-sync)
* [Motivation](#motivation)
* [Contributing](#contributing)

---

## Start Guide

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
mkdir rpsync
cd rpsync
```

> **IMPORTANT**
> If you plan to sync Instagram data, complete the [Instagram Sync Setup](#setup-for-instagram-sync) section first.

---

## Environment Configuration

Create a `.env` file in the project root using the template below:

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

## Setup for Telegram Sync

### Steps

1. Head to [https://my.telegram.org/apps](https://my.telegram.org/apps) and create an app.

2. Copy the **API ID** and **API Hash** and save it for later to use in "Add source" process during the setup.

3. In the Telegram App, navigate to [**Botfather**](https://t.me/BotFather) and create a new bot.

4. Copy the **Bot Token** and save it for later to use in "Add source" process during the setup.

---

## Motivation

This application was created to provide creators with a simple, privacy‑respecting way to analyze their social media presence.

The current release is **alpha** and focuses on manual workflows. Support for additional platforms and automation is planned.

---

## Contributing

Contributions are welcome.

* Submit feature requests or bugs via GitHub Issues
* Open pull requests for improvements or new features

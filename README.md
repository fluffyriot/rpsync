# RPSync

<img width="1280" height="640" alt="rpsync_banner" src="https://github.com/user-attachments/assets/8e35a1b0-e4b7-4a4d-8644-f65e430635ce" />

---

**RPSync** is your personal, privacy-first command center for social media analytics. Run this application locally to collect, visualize, and own your statistics from Instagram, TikTok, Youtube, Bluesky, and more.

## Key Features

*   **100% Local & Private**: Your data stays on your machine.
*   **Unified Dashboard**: Quickly Visualize your posts on the simple dashboard.
*   **Data Ownership**: Export seamlessly to NocoDB or CSV.
*   **Free & Open Source**: No subscriptions, no hidden fees.

---

## Getting Started

### Prerequisites
*   Docker & Docker Compose
*   OpenSSL (Pre-installed on most Linux/WSL systems)
*   **Windows Users**: Install [WSL2](https://learn.microsoft.com/en-us/windows/wsl/install) first.

### Option 1: Automated Installation (Recommended)

The easiest way to get up and running.

1.  **Run the install script:**
    ```bash
    curl -o install.sh https://raw.githubusercontent.com/fluffyriot/rpsync/refs/heads/main/install.sh && sudo chmod +x install.sh && ./install.sh
    ```
2.  **Follow the prompts** to configure:
    *   Deployment Type (Local vs. Public)
    *   IP Address / Domain
    *   Secure Keys (Auto-generated)
    *   Web Server (Caddy) & HTTPS

3.  **Access the App**: Open the URL provided at the end of the script (e.g., `https://192.168.1.50:8443`).

> **Note**: For local installations with self-signed certificates, accept the browser security warning ("Advanced" -> "Proceed").

### Option 2: Manual Installation

<details>
<summary>Click to expand manual setup instructions</summary>

#### 1. Setup Directory
```bash
mkdir rpsync && cd rpsync
```

#### 2. Environment Configuration
Create a `.env` file:
```env
POSTGRES_DB=rpsync-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=password123
POSTGRES_PORT=5435
POSTGRES_HOST=db
POSTGRES_SSLMODE=disable

APP_PORT=22347
HTTP_PORT=8081
HTTPS_PORT=8443
GIN_MODE=debug

LOCAL_IP=
DOMAIN_NAME=
TOKEN_ENCRYPTION_KEY= # generate with: openssl rand -base64 32
OAUTH_ENCRYPTION_KEY= # generate with: openssl rand -base64 32
SESSION_KEY=          # generate with: openssl rand -base64 32
```

#### 3. Docker Compose
Create `docker-compose.yml`:
```yaml
version: "3.9"
services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    container_name: rpsync_db
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
    container_name: rpsync_app
    env_file: .env
    environment:
       HOME: /home/appuser
    depends_on:
      - db
    volumes:
      - ./outputs:/app/outputs
    # ports:
    #   - "${APP_PORT}:${APP_PORT}" # Uncomment for local access

  caddy:
    image: caddy:latest
    container_name: rpsync_caddy
    restart: unless-stopped
    env_file: .env
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - ./certs:/certs
    ports:
      - "${HTTP_PORT}:80"
      - "${HTTPS_PORT}:443"
    depends_on:
      - app

volumes:
  db_data:
```

#### 4. HTTPS (Caddyfile)
Create a `Caddyfile`. Replace variables with actual values.

**For Local (Self-Signed):**
1. Generate certs: `openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout certs/server.key -out certs/server.crt -subj "/CN=${LOCAL_IP}"`
2. Caddyfile content:
```
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}
:${HTTPS_PORT} {
    tls /certs/server.crt /certs/server.key
    reverse_proxy app:${APP_PORT}
}
```

**For Public (Let's Encrypt):**
```
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}
${DOMAIN_NAME} {
    tls your-email@example.com
    reverse_proxy app:${APP_PORT}
}
```

#### 5. Run it
```bash
docker compose up -d
```
</details>

---

## Configuration

### Environment Variables
| Variable | Description |
| :--- | :--- |
| `POSTGRES_...` | Database configuration. Ensure it matches `docker-compose.yml`. |
| `APP_PORT` | Internal app port (default `22347`). |
| `HTTP_PORT` / `HTTPS_PORT` | Caddy external ports. Use `80`/`443` for public deployment. |
| `LOCAL_IP` | Required for local self-signed certificates. |
| `DOMAIN_NAME` | Required for public deployment (Let's Encrypt). |
| `GIN_MODE` | Set to `debug` for detailed server logs, `release` for production. |
| `*_KEY` | Security keys. Generate using `openssl rand -base64 32`. |

---

## Platform Setup

### Instagram Sync
Requires a Facebook Page linked to an Instagram Business/Creator account.

1.  **Create App**: Go to [Meta for Developers](https://developers.facebook.com/apps/), create an app ("Manage everything on your Page").
2.  **Settings**: Get **App ID** and **App Secret**.
3.  **Facebook Login**: Add Valid OAuth Redirect URI: `https://YOUR_IP:PORT_OR_DOMAIN/auth/facebook/callback`.
4.  **Permissions**: Enable `pages_show_list`, `instagram_basic`, `instagram_manage_insights`, `instagram_manage_comments`.

### Telegram Sync
1.  **Create App**: Go to [my.telegram.org](https://my.telegram.org/apps) to get **API ID** and **API Hash**.
2.  **Create Bot**: Talk to [@BotFather](https://t.me/BotFather) to get a **Bot Token**.

### TikTok Sync (Cloud/Public deployment)
Due to TikTok limitations, to enable TikTok sync you need to deploy the app locally first, connect TikTok as a source, and then use the app to export the cookies JSON file. Then, you can import the cookies JSON file into the cloud deployment.

---

## Security & Administration

### User Management (CLI)
Run these commands inside the container or via `docker exec`:

*   **Reset Password**: `./rpsync --reset-password --username <username>`
*   **Reset 2FA**: `./rpsync --reset-2fa --username <username>`

### Authentication Features
*   **Password Policy**: Min 8 chars, uppercase, lowercase, number, special char.
*   **2FA (TOTP)**: Enable in Settings using Google Authenticator / Authy.
*   **Passkeys**: Biometric login (TouchID/FaceID). *Note: Requires HTTPS (or localhost).*

---

## 🤝 Contributing

Contributions are welcome! Please open an issue or submit a PR on GitHub.

*   **Issues**: Report bugs or request features.
*   **Pull Requests**: Submit improvements.


# RPSync

<img width="1280" height="675" alt="RPSync - Promo" src="https://github.com/user-attachments/assets/42fdd21f-e4c0-4452-9fe9-e8b8bcd41be2" />

---

**RPSync** is your personal, privacy-first command center for social media analytics. Run this application locally to collect, visualize, and own your statistics from Instagram, TikTok, Youtube, Bluesky, and more.

## Key Features

*   **100% Local & Private**: Your data stays on your machine.
*   **Unified Dashboard**: Quickly Visualize your posts on the simple dashboard.
*   **Data Ownership**: Export seamlessly to NocoDB or CSV.
*   **Free & Open Source**: No subscriptions, no hidden fees.

## Supported Platforms

### Social Media - Fetch
| Platform | Native API | Public Web Scraping | Logged In Web Scraping | Profile Stats | Posts Stats |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Instagram | ✅ | ❌ | ❌ | ✅ | ✅ |
| TikTok | ❌ | ❌ | ✅ | ✅ | ✅ |
| Youtube | ✅ | ❌ | ❌ | ✅ | ✅ |
| Bluesky | ✅ | ❌ | ❌ | ✅ | ✅ |
| Mastodon | ✅ | ❌ | ❌ | ✅ | ✅ |
| Telegram | ✅ | ✅ | ❌ | ✅ | ✅ |
| Discord | ✅ | ❌ | ❌ | ✅ | ✅ |
| BadPups.com | ❌ | ✅ | ❌ | ✅ | ✅ |
| Murrtube.net | ❌ | ✅ | ❌ | ✅ | ✅ |
| FurTrack.com | ❌ | ✅ | ❌ | ✅ | ✅ |

### Website Stats - Fetch
| Website | Native API | Website Visitors | Page Views |
| :--- | :--- | :--- | :--- |
| Google Analytics | ✅ | ✅ | ✅ |

### Data - Push
| Target | Native API | Social Profile Stats | Social Posts Stats | Website Stats |
| :--- | :--- | :--- | :--- | :--- |
| NocoDB | ✅ | ✅ | ✅ | ✅ |
| CSV | N/A | ✅ | ✅ | ✅ |

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
mkdir -p outputs && sudo chmod 777 outputs
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
      - "${HTTP_PORT}:${HTTP_PORT}"
      - "${HTTPS_PORT}:${HTTPS_PORT}"
    depends_on:
      - app

  watchtower:
    image: nickfedor/watchtower
    container_name: rpsync_watchtower
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --interval 14400 --cleanup rpsync_app

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

</details>

---

## Platform Setup

### Setup for Instagram Sync

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

#### Steps

1. Go to **Meta for Developers** and create an account:
   [https://developers.facebook.com/apps/](https://developers.facebook.com/apps/)

2. Create a new app.

   * Select:

     * *Manage Messaging and content on Instagram*
     * *Manage everything on your Page*

3. In the App Dashboard, navigate to **Settings → Basic**.

4. Copy the **App ID** and **App Secret**. You will use them later at the source setup in the app.

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

11. Copy and save the numeric Instagram Page ID displayed. You will use them later at the source setup in the app.

---

### Telegram Sync
1.  **Create App**: Go to [my.telegram.org](https://my.telegram.org/apps) to get **API ID** and **API Hash**.
2.  **Create Bot**: Talk to [@BotFather](https://t.me/BotFather) to get a **Bot Token**.

---

### TikTok Sync (Cloud/Public deployment)
Due to TikTok limitations, to enable TikTok sync you need to deploy the app locally first, connect TikTok as a source, and then use the app to export the cookies JSON file. Then, you can import the cookies JSON file into the cloud deployment.

---

### Discord Sync
Sync messages from Discord text channels and threads from Discord forum channels.

#### 1. Create Discord Application
1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **New Application**, give it a name, accept ToS, and click **Create**
3. Navigate to the **Bot** section in the left sidebar

#### 2. Configure Bot Settings
1. Click **Reset Token** to generate a new bot token
2. **Copy and save the token** - you'll need this for RPSync configuration
3. Under **Privileged Gateway Intents**, enable:
   - **Server Members Intent** (required for member count/followers)
   - **Message Content Intent** (required to read message text)

#### 3. Set Bot Permissions
1. Navigate to **OAuth2** → **URL Generator** in the left sidebar
2. Under **Scopes**, select:
   - `bot`
3. Under **Bot Permissions**, select:
   - **View Channels** (required to see channels)
   - **Read Message History** (required to fetch messages)
4. Copy the generated URL at the bottom

#### 4. Invite Bot to Your Server
1. Open the copied URL in your browser
2. Select your Discord server from the dropdown
3. Click **Authorize** and complete the CAPTCHA
4. Verify the bot appears in your server's member list

#### 5. Get Server and Channel IDs
1. Enable **Developer Mode** in Discord:
   - User Settings → App Settings → Advanced → Enable Developer Mode
2. Right-click your server icon → **Copy Server ID**
3. Right-click each channel you want to sync → **Copy Channel ID**

#### 6. Configure in RPSync
When adding a Discord source in RPSync:
- **Bot Token**: The token from step 2
- **Server ID**: The server ID from step 5
- **Channel IDs**: Comma-separated list of channel IDs (e.g., `123456789,987654321`)

**Supported Channel Types:**
- **Text Channels**: Syncs messages as posts
- **Forum Channels**: Syncs threads as posts (thread title = content, message count = likes)

---

## Security & Administration

### User Management (CLI)
Run these commands inside the container or via `docker exec`:

*   **Reset Password**: `./rpsync --reset-password --username <username>`
*   **Reset 2FA**: `./rpsync --reset-2fa --username <username>`

### Authentication Features
*   **Password Policy**: Min 8 characters, must contain uppercase, lowercase, number, and special character.
*   **2FA (TOTP)**: Enable in Settings using Google Authenticator / Authy.
*   **Passkeys**: Biometric login (TouchID/FaceID). *Note: Requires public domain deployment.*

---

## Known Limitations
*   **Integer Overflow Protection**: Social media stats (Likes, Views, Reposts) that exceed the maximum 32-bit integer value (2,147,483,647) will be clamped to this maximum value to prevent errors.

---

## Contributing

Contributions are welcome! Please open an issue or submit a PR on GitHub.

*   **Issues**: Report bugs or request features.
*   **Pull Requests**: Submit improvements.


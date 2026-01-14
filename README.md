# commission-tracker
Gather your online presence stats in a small local database.

## Quick Start

1. Ensure you have Docker installed.

Run the following commands in the terminal.

```
docker --version
docker compose version
```

2. Create the working directory.

```
mkdir commission-tracker
cd commission-tracker
```

3. (Optional) If you plan to syncronize data from your Instagram, go to the [Instagram section](#setup-for-instagram-sync) first.

4. Create `.env` file in your app folder. Use the template below.

```
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

- `POSTGRES_*` details of the database to be created. Make sure to change the password to have it secured. Other fields can stay as they are.
- `*_PORT` ports to be used in the communication. Ensure your app port and https ports are unique.
- `INSTAGRAM_API_VERSION` v.24.0 of Facebook Graph API for Instagram has been tested. Other versions could cause unexpected issues.
- `TOKEN_ENCRYPTION_KEY` is used to encrypt and decrypt tokens storred in your local database. Use terminal command `openssl rand -base64 32` to generate a key.
- `OAUTH_ENCRYPTION_KEY` is used to encrypt the OAuth URLs for Facebook login. Use terminal command `openssl rand -base64 32` to generate a key.
- `POSTGRES_PORT` is the unique port for Database to be used in Docker settings. 
- `APP_PORT` is the unique port to be used for your App HTTP server.
- `HTTPS_PORT` is the unique port to be used for you to communicate with your App over HTTPs.
- `FACEBOOK_APP_*` details you need to obtain from Facebook for Developers to sync your Instagram data. Process explained further in the guide.

5. Create `docker-compose.yml` file using the template below in your app folder. No changes needed.

```
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

6. Create Caddyfile.template and copy the code below:

```
# HTTP redirect to HTTPS
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}

# HTTPS site with self-signed certificate
${LOCAL_IP}:${HTTPS_PORT} {
    tls /certs/server.crt /certs/server.key
    reverse_proxy app:${APP_PORT}
}
```

7. Using Terminal, go to your app folder and:

  1. Run this to generate Caddyfile:
```
export $(grep -v '^#' .env | xargs)
envsubst < Caddyfile.template > Caddyfile
```
  2. Create `certs` folder and generate your certificates using following command:
```
IP=${LOCAL_IP:-127.0.0.1}
mkdir -p certs
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout certs/server.key -out certs/server.crt -subj "/CN=${IP}" -addext "subjectAltName=IP:${IP}"
echo "Self-signed certificate generated for ${IP} in ./certs/"
```
  3. Run `docker compose pull` to get all Docker images.
  

6. Start the application using `docker compose up -d`

7. Open your browser to: `https://LOCAL_IP:HTTPS_PORT` and replace ip and port with the values from .env file.


## Motivation

I have created this app to simplify for myself and other digital creators analysis of their social media presence. Tools available online are usually expensive and data-intrusive while this app is running locally for you.

In the current alpha release it is fully manual, though more websites and automations are coming soon.

## Usage

Follow the simple Web UI for the available features.

To enable Instagram fetch, you need to create your own app using Meta Developers portal and obtain API key from there.

## Contributing

Feel free to suggest features using Github issues or develop them and create pull requests.


## Setup for Instagram Sync

> [!IMPORTANT]
> To be able to use Sync for Instagram, your Instagram page should be a [business or creator account](https://help.instagram.com/1980623166138346/) and [connected to a Facebook page](https://www.facebook.com/help/instagram/790156881117411).

1. Head to [Meta for Developers](https://developers.facebook.com/apps/) and sign up.
2. Create a new app.
  - Use "Manage Messaging and content on Instagram" and "Manage everything on your page"
3. In the App dashboard, go to Settings -> Basic
4. Copy App ID and App secret (you will need them in your .env file)
5. Go to "Facebook Login for Business"
  - Under "Client OAuth settings" in "Valid OAuth Redirect URIs" and URL: `https://LOCAL_IP:HTTPS_PORT/auth/facebook/callback`. Replace IP and Port with ones you give to your .env file.
6. Go to "Use cases" and click on Edit button on Instagram use case
7. Go to "API setup with Facebook Login" and use "Add permissions" button under "Manage content" section.
8. Under "Permissions and features" find and add "Manage insights" and "Manage comments"
8. Go to [Graph Explorer](https://developers.facebook.com/tools/explorer/) and click "Generate Token".
9. Tick your Page and your Instagram profile in the new Facebook window and **COPY** the number under your Instagram page and save it for later.

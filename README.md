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

3. Create `.env` file. Use the template below.

```
POSTGRES_DB=commission-tracker-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=password123
PORT=8080
POSTGRES_HOST=db

INSTAGRAM_API_VERSION=24.0
TOKEN_ENCRYPTION_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx // Key used for encrypting tokens. Use terminal command "openssl rand -base64 32" to generate a secure key.
POSTGRES_PORT=5435
APP_PORT=22347
```

Keep the first part intact. 

- `INSTAGRAM_API_VERSION` v.24.0 of Instagram API has been tested. Other versions could cause unexpected issues.
- `TOKEN_ENCRYPTION_KEY` is used to encrypt and decrypt tokens storred in your local database. Use terminal command `openssl rand -base64 32` to generate a key.
- `POSTGRES_PORT` is the unique port to be used in Docker settings. 
- `APP_PORT` is the unique port to be used to communicate with your app after installation.

3. Create `docker-compose.yml` file using the template below. No changes needed.

```
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - db_data:/var/lib/postgresql/data
    restart: unless-stopped

  app:
    image: fluffyriot/commission-tracker:latest
    depends_on:
      - db
    env_file:
      - .env
    ports:
      - "${APP_PORT}:8080"
    volumes:
      - ./output:/app/output
    restart: unless-stopped

volumes:
  db_data:

```

4. Start the application using `docker compose up -d`

5. Open your browser to: `http://localhost:22347` where `22347` is your `APP_TOKEN`


## Motivation

I have created this app to simplify for myself and other digital creators analysis of their social media presence. Tools available online are usually expensive and data-intrusive while this app is running locally for you.

In the current alpha release it is fully manual, though more websites and automations are coming soon.

## Usage

Follow the simple Web UI for the available features.

To enable Instagram fetch, you need to create your own app using Meta Developers portal and obtain API key from there.

## Contributing

Feel free to suggest features using Github issues or develop them and create pull requests.

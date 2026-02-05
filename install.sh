#!/bin/bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN} Welcome to the RPSync Installer! ${NC}"
echo "-----------------------------------"

echo -e "\n${YELLOW} Checking dependencies... ${NC}"

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

install_package() {
    PACKAGE=$1
    if command_exists apt-get; then
        sudo apt-get update && sudo apt-get install -y "$PACKAGE"
    elif command_exists dnf; then
        sudo dnf install -y "$PACKAGE"
    elif command_exists yum; then
        sudo yum install -y "$PACKAGE"
    elif command_exists pacman; then
        sudo pacman -S --noconfirm "$PACKAGE"
    elif command_exists apk; then
        sudo apk add "$PACKAGE"
    else
        echo -e "${RED} Error: Package manager not found. Please install $PACKAGE manually. ${NC}"
        return 1
    fi
}

if ! command_exists docker; then
    echo -e "${YELLOW} Docker is not installed. ${NC}"
    read -p "Would you like to install Docker automatically? (y/N): " INSTALL_DOCKER
    if [[ "$INSTALL_DOCKER" =~ ^[Yy]$ ]]; then
        echo "Installing Docker..."
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        rm get-docker.sh
        echo "Adding current user to docker group..."
        sudo usermod -aG docker $USER
        echo -e "${YELLOW} NOTE: You may need to log out and back in for Docker group changes to take effect. ${NC}"
    else
        echo -e "${RED} Docker is required. Please install it manually. ${NC}"
        exit 1
    fi
fi

if ! command_exists openssl; then
    echo -e "${YELLOW} OpenSSL is not installed. ${NC}"
    read -p "Would you like to install OpenSSL automatically? (y/N): " INSTALL_OPENSSL
    if [[ "$INSTALL_OPENSSL" =~ ^[Yy]$ ]]; then
        install_package openssl
    else
        echo -e "${RED} OpenSSL is required. Please install it manually. ${NC}"
        exit 1
    fi
fi

echo -e "${GREEN} Dependencies checked. ${NC}"

echo -e "\n${YELLOW} Setting up installation directory (~/rps)... ${NC}"
INSTALL_DIR="$HOME/rps"
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR" || { echo -e "${RED} Failed to create or enter installation directory. ${NC}"; exit 1; }
echo "Working directory set to: $PWD"

echo -e "\n${YELLOW} Configuration Setup ${NC}"

DEFAULT_IP=$(hostname -I | tr ' ' '\n' | grep -E '^(192\.168|10\.|172\.(1[6-9]|2[0-9]|3[0-1]))\.' | head -n 1)
if [ -z "$DEFAULT_IP" ]; then
    DEFAULT_IP=$(hostname -I | awk '{print $1}')
fi
if [ -z "$DEFAULT_IP" ]; then
    DEFAULT_IP="127.0.0.1"
fi

read -p "Local IP Address. Press Enter to confirm default or type a new one [${DEFAULT_IP}]: " LOCAL_IP
LOCAL_IP=${LOCAL_IP:-$DEFAULT_IP}

echo -e "\nSelect deployment type:"
echo "1) Local (Self-Signed Certificates, for local networks)"
echo "2) Public (Requires Public Domain Name)"
read -p "Choose [1]: " DEPLOY_TYPE
DEPLOY_TYPE=${DEPLOY_TYPE:-1}

DOMAIN_NAME=""
if [ "$DEPLOY_TYPE" == "2" ]; then
    while [ -z "$DOMAIN_NAME" ]; do
        read -p "Enter your Public Domain Name: " DOMAIN_NAME
        if [ -z "$DOMAIN_NAME" ]; then
            echo -e "${RED} Error: Public Domain Name is required for Public deployment. ${NC}"
        fi
    done
fi

if [ "$DEPLOY_TYPE" == "2" ]; then
    DEFAULT_HTTP_PORT=80
    DEFAULT_HTTPS_PORT=443
    DEFAULT_PG_PORT=5432
else
    DEFAULT_HTTP_PORT=8081
    DEFAULT_HTTPS_PORT=8443
    DEFAULT_PG_PORT=5435
fi

DEFAULT_APP_PORT=22347
read -p "App Port. Press Enter to confirm default or type a new one [${DEFAULT_APP_PORT}]: " APP_PORT
APP_PORT=${APP_PORT:-$DEFAULT_APP_PORT}

read -p "HTTP Port. Press Enter to confirm default or type a new one [${DEFAULT_HTTP_PORT}]: " HTTP_PORT
HTTP_PORT=${HTTP_PORT:-$DEFAULT_HTTP_PORT}

read -p "HTTPS Port. Press Enter to confirm default or type a new one [${DEFAULT_HTTPS_PORT}]: " HTTPS_PORT
HTTPS_PORT=${HTTPS_PORT:-$DEFAULT_HTTPS_PORT}

read -p "Database Port. Press Enter to confirm default or type a new one [${DEFAULT_PG_PORT}]: " POSTGRES_PORT
POSTGRES_PORT=${POSTGRES_PORT:-$DEFAULT_PG_PORT}

echo -e "\n${YELLOW} Generating .env file... ${NC}"

GENERATE_ENV=true

if [ "$GENERATE_ENV" = true ]; then
    TOKEN_KEY=$(openssl rand -base64 32)
    OAUTH_KEY=$(openssl rand -base64 32)
    SESSION_KEY=$(openssl rand -base64 32)
    DB_PASSWORD=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 24)

    cat > .env <<EOF
POSTGRES_DB=rpsync-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=${DB_PASSWORD}
POSTGRES_PORT=${POSTGRES_PORT}
POSTGRES_HOST=db
POSTGRES_SSLMODE=disable

APP_PORT=${APP_PORT}
HTTP_PORT=${HTTP_PORT}
HTTPS_PORT=${HTTPS_PORT}

LOCAL_IP=${LOCAL_IP}
DOMAIN_NAME=${DOMAIN_NAME}

TOKEN_ENCRYPTION_KEY=${TOKEN_KEY}
OAUTH_ENCRYPTION_KEY=${OAUTH_KEY}
SESSION_KEY=${SESSION_KEY}
EOF
    echo -e "${GREEN} .env file created. ${NC}"
fi

echo -e "\n${YELLOW} Setting up Web Server (Caddy)... ${NC}"

mkdir -p certs

if [ "$DEPLOY_TYPE" == "1" ]; then
    echo "Generating self-signed certificate for ${LOCAL_IP}..."
    openssl req -x509 -nodes -days 365 \
      -newkey rsa:2048 \
      -keyout certs/server.key \
      -out certs/server.crt \
      -subj "/CN=${LOCAL_IP}" \
      -addext "subjectAltName=IP:${LOCAL_IP}" >/dev/null 2>&1
    
    SITE_ADDRESS=":${HTTPS_PORT}"
    TLS_CONFIG="tls /certs/server.crt /certs/server.key"
else
    SITE_ADDRESS="${DOMAIN_NAME}"
    
    echo -e "\n${YELLOW} TLS Configuration (Let's Encrypt) ${NC}"

    TLS_CONFIG="" 
     
    LE_EMAIL=""
    while [ -z "$LE_EMAIL" ]; do
        read -p "Enter your email for Let's Encrypt registration: " LE_EMAIL
        if [ -z "$LE_EMAIL" ]; then
            echo -e "${RED} Error: Email is required for Let's Encrypt registration. ${NC}"
        fi
    done
    TLS_CONFIG="tls ${LE_EMAIL}"
     
    echo -e "${GREEN} Configured for Let's Encrypt. ${NC}"
    
    echo -e "${YELLOW} Note: For public domains using non-standard ports, Let's Encrypt validation may require DNS-01 challenge or port forwarding. ${NC}"
fi

cat > Caddyfile <<EOF
# Redirect HTTP to HTTPS
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}

# HTTPS site
${SITE_ADDRESS} {
    ${TLS_CONFIG}
    reverse_proxy app:${APP_PORT}
}
EOF
echo -e "${GREEN} Caddyfile created. ${NC}"

echo -e "\n${YELLOW} Generating Docker Compose configuration... ${NC}"

APP_PORTS_CONFIG=""
if [ "$DEPLOY_TYPE" == "1" ]; then
    APP_PORTS_CONFIG="    ports:
      - \"\${APP_PORT}:\${APP_PORT}\""
fi

cat > docker-compose.yml <<EOF
version: "3.9"

services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped
    container_name: rpsync_db
    environment:
      POSTGRES_USER: \${POSTGRES_USER}
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD}
      POSTGRES_DB: \${POSTGRES_DB}
    volumes:
      - db_data:/var/lib/postgresql/data
    ports:
      - "\${POSTGRES_PORT}:5432"

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
${APP_PORTS_CONFIG}

  caddy:
    image: caddy:latest
    container_name: rpsync_caddy
    restart: unless-stopped
    env_file: .env
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - ./certs:/certs
    ports:
      - "\${HTTP_PORT}:\${HTTP_PORT}"
      - "\${HTTPS_PORT}:\${HTTPS_PORT}"
    depends_on:
      - app

volumes:
  db_data:
EOF

echo -e "${GREEN} docker-compose.yml generated. ${NC}"

echo -e "\n${YELLOW} Setting up permissions... ${NC}"
mkdir -p outputs
sudo chmod 777 outputs
echo -e "${GREEN} Permissions set for outputs directory. ${NC}"

echo -e "\n${YELLOW} Ready to start! ${NC}"
read -p "Do you want to start the application now? (Y/n): " START_DOCKER
START_DOCKER=${START_DOCKER:-Y}

if [[ "$START_DOCKER" =~ ^[Yy]$ ]]; then
    echo "Starting Docker containers..."
    docker compose up -d
    echo -e "\n${GREEN}Application started!${NC}"
    if [ "$DEPLOY_TYPE" == "1" ]; then
        echo -e "Access it at: https://${LOCAL_IP}:${HTTPS_PORT}"
    else
        echo -e "Access it at: https://${DOMAIN_NAME}"
    fi
    echo -e "Note: Since you used a self-signed certificate, your browser will warn you. You can safely accept the risk for local usage."
else
    echo "You can start the application later by running: docker compose up -d"
fi

echo -e "\n${GREEN} Installation Complete! ${NC}"

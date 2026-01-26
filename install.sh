#!/bin/bash

# RPSync Installation Script
# This script guides the user through the setup process.

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Welcome to the RPSync Installer!${NC}"
echo "-----------------------------------"

# 1. Dependency Checks
echo -e "\n${YELLOW}Checking dependencies...${NC}"

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
        echo -e "${RED}Error: Package manager not found. Please install $PACKAGE manually.${NC}"
        return 1
    fi
}

if ! command_exists docker; then
    echo -e "${YELLOW}Docker is not installed.${NC}"
    read -p "Would you like to install Docker automatically? (y/N): " INSTALL_DOCKER
    if [[ "$INSTALL_DOCKER" =~ ^[Yy]$ ]]; then
        echo "Installing Docker..."
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        rm get-docker.sh
        echo "Adding current user to docker group..."
        sudo usermod -aG docker $USER
        echo -e "${YELLOW}NOTE: You may need to log out and back in for Docker group changes to take effect.${NC}"
    else
        echo -e "${RED}Docker is required. Please install it manually.${NC}"
        exit 1
    fi
fi

if ! command_exists openssl; then
    echo -e "${YELLOW}OpenSSL is not installed.${NC}"
    read -p "Would you like to install OpenSSL automatically? (y/N): " INSTALL_OPENSSL
    if [[ "$INSTALL_OPENSSL" =~ ^[Yy]$ ]]; then
        install_package openssl
    else
        echo -e "${RED}OpenSSL is required. Please install it manually.${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}Dependencies checked.${NC}"

echo -e "\n${YELLOW}Configuration Setup${NC}"

DEFAULT_IP=$(hostname -I | awk '{print $1}')
if [ -z "$DEFAULT_IP" ]; then
    DEFAULT_IP="127.0.0.1"
fi

read -p "Enter the Local IP Address [${DEFAULT_IP}]: " LOCAL_IP
LOCAL_IP=${LOCAL_IP:-$DEFAULT_IP}

echo -e "\nSelect deployment type:"
echo "1) Local (Self-Signed Certificates, for home networks)"
echo "2) Public (Let's Encrypt / Public Domain / Public IP)"
read -p "Choose [1]: " DEPLOY_TYPE
DEPLOY_TYPE=${DEPLOY_TYPE:-1}

DOMAIN_NAME=""
if [ "$DEPLOY_TYPE" == "2" ]; then
    read -p "Enter your Public Domain Name (leave empty to use IP ${LOCAL_IP}): " DOMAIN_NAME
    if [ -z "$DOMAIN_NAME" ]; then
        echo -e "${YELLOW}No domain provided. Using IP address: ${LOCAL_IP}${NC}"
        DOMAIN_NAME=${LOCAL_IP}
    fi
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

read -p "Enter HTTP Port [${DEFAULT_HTTP_PORT}]: " HTTP_PORT
HTTP_PORT=${HTTP_PORT:-$DEFAULT_HTTP_PORT}

read -p "Enter HTTPS Port [${DEFAULT_HTTPS_PORT}]: " HTTPS_PORT
HTTPS_PORT=${HTTPS_PORT:-$DEFAULT_HTTPS_PORT}

read -p "Enter Database Port [${DEFAULT_PG_PORT}]: " POSTGRES_PORT
POSTGRES_PORT=${POSTGRES_PORT:-$DEFAULT_PG_PORT}

echo -e "\n${YELLOW}Generating .env file...${NC}"

if [ -f .env ]; then
    read -p ".env file already exists. Overwrite? (y/N): " OVERWRITE_ENV
    if [[ "$OVERWRITE_ENV" =~ ^[Yy]$ ]]; then
        GENERATE_ENV=true
    else
        GENERATE_ENV=false
        echo "Skipping .env generation."
    fi
else
    GENERATE_ENV=true
fi

if [ "$GENERATE_ENV" = true ]; then
    TOKEN_KEY=$(openssl rand -base64 32)
    OAUTH_KEY=$(openssl rand -base64 32)
    SESSION_KEY=$(openssl rand -base64 32)
    DB_PASSWORD=$(openssl rand -base64 16)

    cat > .env <<EOF
POSTGRES_DB=rpsync-db
POSTGRES_USER=local-user-ctd
POSTGRES_PASSWORD=${DB_PASSWORD}
POSTGRES_PORT=${POSTGRES_PORT}
POSTGRES_HOST=db
POSTGRES_SSLMODE=disable

APP_PORT=22347
HTTP_PORT=${HTTP_PORT}
HTTPS_PORT=${HTTPS_PORT}
GIN_MODE=release

LOCAL_IP=${LOCAL_IP}
DOMAIN_NAME=${DOMAIN_NAME}

TOKEN_ENCRYPTION_KEY=${TOKEN_KEY}
OAUTH_ENCRYPTION_KEY=${OAUTH_KEY}
SESSION_KEY=${SESSION_KEY}
EOF
    echo -e "${GREEN}.env file created.${NC}"
fi

echo -e "\n${YELLOW}Setting up Web Server (Caddy)...${NC}"

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
    TLS_CONFIG="tls internal"
    echo -e "${YELLOW}Note: For public domains using non-standard ports, Let's Encrypt validation may require DNS-01 challenge or port forwarding.${NC}"
fi

cat > Caddyfile <<EOF
# Redirect HTTP to HTTPS
:${HTTP_PORT} {
    redir https://{host}:${HTTPS_PORT}{uri} permanent
}

# HTTPS site
${SITE_ADDRESS} {
    ${TLS_CONFIG}
    reverse_proxy app:22347
}
EOF
echo -e "${GREEN}Caddyfile created.${NC}"

echo -e "\n${YELLOW}Ready to start!${NC}"
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

echo -e "\n${GREEN}Installation Complete!${NC}"

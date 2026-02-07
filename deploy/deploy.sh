#!/bin/bash
# Ensoul deployment script
# Usage: ./deploy.sh [dev|prod]

set -e

MODE=${1:-dev}
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Ensoul Deployment ==="
echo "Mode: $MODE"
echo "Directory: $PROJECT_DIR"
echo ""

# Check .env exists
if [ ! -f .env ]; then
    echo "ERROR: .env file not found!"
    echo "Run: cp deploy/.env.example .env"
    echo "Then edit .env with your production values."
    exit 1
fi

# Source .env for validation
source .env

# Validate critical env vars
if [ -z "$DB_PASSWORD" ] || [ "$DB_PASSWORD" = "change_me_to_a_strong_password" ]; then
    echo "WARNING: DB_PASSWORD is not set or still default!"
fi

if [ -z "$LLM_API_KEY" ]; then
    echo "WARNING: LLM_API_KEY is not set. AI features will use fallback mode."
fi

if [ -z "$PLATFORM_PRIVATE_KEY" ]; then
    echo "WARNING: PLATFORM_PRIVATE_KEY is not set. On-chain features will be disabled."
fi

echo ""
echo "Building and starting services..."

if [ "$MODE" = "prod" ] || [ "$MODE" = "production" ]; then
    # Check SSL certs
    if [ ! -f deploy/certs/fullchain.pem ]; then
        echo "ERROR: SSL certificates not found in deploy/certs/"
        echo "See deploy/DEPLOY.md for SSL setup instructions."
        exit 1
    fi

    docker compose --profile production up -d --build
    echo ""
    echo "Production services started with Nginx + SSL."
else
    docker compose up -d --build
    echo ""
    echo "Development services started."
    echo "  Frontend: http://localhost:3000"
    echo "  API:      http://localhost:8080"
    echo "  DB:       localhost:5432"
fi

echo ""
echo "Check status: docker compose ps"
echo "View logs:    docker compose logs -f"
echo ""

# Wait for health check
echo "Waiting for API health check..."
sleep 5

API_URL="http://localhost:8080"
if [ "$MODE" = "prod" ] || [ "$MODE" = "production" ]; then
    API_URL="https://api.ensoul.ac"
fi

if curl -sf "$API_URL/api/health" > /dev/null 2>&1; then
    echo "✓ API is healthy!"
else
    echo "✗ API health check failed. Check logs: docker compose logs server"
fi

echo ""
echo "=== Deployment Complete ==="

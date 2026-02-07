# Ensoul Deployment Guide

## Quick Deploy (Docker Compose)

### 1. Prerequisites

- A Linux server with Docker and Docker Compose installed
- A domain name (e.g., `ensoul.ac`) with DNS pointing to your server
- SSL certificates (use Let's Encrypt / Certbot)

### 2. Setup

```bash
# Clone the repository
git clone https://github.com/ensoul-labs/ensoul.git
cd ensoul

# Copy and edit the environment file
cp deploy/.env.example .env
nano .env
```

Fill in all required values in `.env`:
- `DB_PASSWORD` — strong password for PostgreSQL
- `PLATFORM_PRIVATE_KEY` — BSC wallet private key (funded with BNB)
- `CLAW_PK_SECRET` — random string for AES encryption
- `LLM_API_KEY` — your OpenAI / DeepSeek API key
- `NEXT_PUBLIC_API_URL` — `https://api.ensoul.ac`

### 3. SSL Certificates

Place your SSL certificates in `deploy/certs/`:
```bash
mkdir -p deploy/certs
# Copy fullchain.pem and privkey.pem
cp /path/to/fullchain.pem deploy/certs/
cp /path/to/privkey.pem deploy/certs/
```

For Let's Encrypt:
```bash
sudo certbot certonly --standalone -d ensoul.ac -d www.ensoul.ac -d api.ensoul.ac
sudo cp /etc/letsencrypt/live/ensoul.ac/fullchain.pem deploy/certs/
sudo cp /etc/letsencrypt/live/ensoul.ac/privkey.pem deploy/certs/
```

### 4. Launch

```bash
# Development (no Nginx/SSL)
docker compose up -d

# Production (with Nginx + SSL)
docker compose --profile production up -d
```

### 5. Verify

```bash
# Check all services are running
docker compose ps

# Check API health
curl https://api.ensoul.ac/api/health

# Check frontend
curl https://ensoul.ac
```

## Manual Deploy (Without Docker)

### Backend

```bash
cd server
cp .env.example .env
# Edit .env with production values

# Build
go build -o ensoul-server .

# Run (or use systemd)
./ensoul-server
```

**Systemd service** (`/etc/systemd/system/ensoul.service`):
```ini
[Unit]
Description=Ensoul API Server
After=postgresql.service

[Service]
Type=simple
WorkingDirectory=/opt/ensoul/server
ExecStart=/opt/ensoul/server/ensoul-server
EnvironmentFile=/opt/ensoul/server/.env
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Frontend

```bash
cd web
npm ci
NEXT_PUBLIC_API_URL=https://api.ensoul.ac npm run build

# Run with PM2 or similar
npx next start -p 3000
```

### Database

```bash
# Create database
sudo -u postgres createuser ensoul
sudo -u postgres createdb -O ensoul ensoul

# The Go server auto-migrates tables on startup
```

## Vercel Deployment (Frontend Only)

1. Connect the `web/` directory to Vercel
2. Set environment variable: `NEXT_PUBLIC_API_URL=https://api.ensoul.ac`
3. Deploy

## Architecture

```
Internet
    │
    ▼
┌─────────┐
│  Nginx  │ (port 80/443)
│  + SSL  │
└────┬────┘
     │
     ├──── ensoul.ac ────▶ Next.js (port 3000)
     │
     └──── api.ensoul.ac ▶ Go API  (port 8080)
                                │
                                ▼
                          PostgreSQL (port 5432)
                                │
                                ▼
                          BNB Chain (ERC-8004)
```

## Monitoring

Check service logs:
```bash
docker compose logs -f server    # Backend logs
docker compose logs -f web       # Frontend logs
docker compose logs -f db        # Database logs
docker compose logs -f nginx     # Nginx logs
```

## Backup

```bash
# Database backup
docker compose exec db pg_dump -U ensoul ensoul > backup_$(date +%Y%m%d).sql

# Database restore
cat backup_20260218.sql | docker compose exec -T db psql -U ensoul ensoul
```

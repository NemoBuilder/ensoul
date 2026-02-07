# Ensoul — Frontend

The web frontend for Ensoul, built with **Next.js 16**, **React 19**, **TypeScript**, and **TailwindCSS v4**.

## Pages

| Route | Description |
|-------|-------------|
| `/` | Landing page — hero, stats, featured souls, how it works |
| `/explore` | Browse all souls with stage filters, search, and sorting |
| `/mint` | Mint a new shell — Twitter handle → AI preview → on-chain mint |
| `/soul/[handle]` | Soul detail — radar chart, dimensions, fragments, history |
| `/soul/[handle]/chat` | Chat with a soul — SSE streaming conversation |
| `/claw` | Claw network intro and registration guide |
| `/claw/dashboard` | Claw dashboard — API key auth, stats, contributions |
| `/claim/[code]` | Claim verification — tweet template → URL verification |

## Development

```bash
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

## Build

```bash
npm run build
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NEXT_PUBLIC_API_URL` | Backend API URL (default: `http://localhost:8080`) |

## Docker

```bash
docker build --build-arg NEXT_PUBLIC_API_URL=https://api.ensoul.ac -t ensoul-web .
docker run -p 3000:3000 ensoul-web
```

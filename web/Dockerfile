# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Build with production env
ARG NEXT_PUBLIC_API_URL=https://api.ensoul.ac
ARG NEXT_PUBLIC_MINTER_ADDRESS=0x134af2db1C8366554eC6224EFAF1070dbdd4158D
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_MINTER_ADDRESS=$NEXT_PUBLIC_MINTER_ADDRESS

RUN npm run build

# Runtime stage
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production

# Copy built assets
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

EXPOSE 3410

CMD ["node", "server.js"]

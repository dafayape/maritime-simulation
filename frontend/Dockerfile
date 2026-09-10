# syntax=docker/dockerfile:1

# ---------- Tahap 1: dependensi (cache layer terpisah dari source) ----------
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# ---------- Tahap 2: build Next.js (output standalone) ----------------------
FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* di-inline ke bundle SAAT BUILD (perilaku Next.js).
#  - kosong  -> mode same-origin: browser memakai origin-nya sendiri; cocok
#    untuk VPS di belakang nginx yang mem-proxy /api/v1 dan /ws ke backend.
#  - berisi  -> alamat backend eksplisit (dev lokal: http://localhost:8080,
#    di-set dari args docker-compose).
ARG NEXT_PUBLIC_API_URL=""
ARG NEXT_PUBLIC_WS_URL=""
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL} \
    NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL} \
    NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---------- Tahap 3: runtime minimal non-root -------------------------------
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

RUN addgroup -g 10001 -S nextjs && adduser -u 10001 -S nextjs -G nextjs

# output "standalone": server.js + node_modules hasil tracing saja —
# image akhir tidak membawa devDependencies maupun source TypeScript.
COPY --from=builder --chown=nextjs:nextjs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nextjs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nextjs /app/public ./public

USER nextjs
EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null 2>&1 || exit 1

CMD ["node", "server.js"]

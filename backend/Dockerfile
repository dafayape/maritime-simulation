# syntax=docker/dockerfile:1

# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Dependency layer first: rebuilds are cache-friendly.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/nodesim ./cmd/nodesim

# ---------- runtime stage ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 simulator

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/nodesim /app/nodesim

USER simulator
EXPOSE 8080

# busybox wget ships with alpine; /healthz is the liveness probe.
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/server"]

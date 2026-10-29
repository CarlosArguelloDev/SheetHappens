# syntax=docker/dockerfile:1
# =========================================================
# SheetHappens - Go Print API & Web Portal (Multi-Stage)
# =========================================================

# --- Stage 1: Builder --------------------------------------------------------
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Download dependencies first (cached layer when go.mod/go.sum are unchanged).
COPY go.mod ./
RUN go mod download

# Copy backend source and build a statically linked Linux binary.
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/sheethappens ./cmd/server

# --- Stage 2: Minimal Runtime ------------------------------------------------
FROM alpine:latest

LABEL org.opencontainers.image.title="sheethappens-print-api" \
      org.opencontainers.image.description="SheetHappens Go print API and bilingual web portal" \
      org.opencontainers.image.source="https://github.com/CarlosArguelloDev/SheetHappens"

# Install CUPS CLI client (`lp`, `lpstat`) and root CA bundle.
RUN apk add --no-cache \
        cups-client \
        ca-certificates \
        tzdata \
 && addgroup -S appuser \
 && adduser -S -G appuser -u 10001 -h /app appuser

WORKDIR /app

# Copy statically compiled binary and static frontend assets.
COPY --from=builder --chown=appuser:appuser /out/sheethappens /app/sheethappens
COPY --chown=appuser:appuser web/ /app/web/

# Ensure temporary print spool directory is writable by appuser.
ENV PORT=8080 \
    CUPS_HOST=cups:631 \
    WEB_DIR=/app/web \
    TMP_DIR=/tmp

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${PORT:-8080}/api/health" || exit 1

ENTRYPOINT ["/app/sheethappens"]

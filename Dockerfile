# Stage 1: Build Frontend Assets
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Backend Binary
FROM golang:1.26-bookworm AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/ethiyo ./cmd/server

# Stage 3: Minimal Production Runtime with Chromium & Tini
FROM debian:bookworm-slim

# Install Chromium runtime dependencies, ca-certificates, tzdata, and tini
RUN apt-get update && apt-get install --no-install-recommends -y \
    chromium \
    chromium-sandbox \
    ca-certificates \
    tzdata \
    tini \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create unprivileged application user
RUN useradd -u 10001 -m -s /bin/bash appuser
WORKDIR /home/appuser

# Copy application artifacts
COPY --from=backend-builder /bin/ethiyo /usr/local/bin/ethiyo
COPY --from=frontend-builder /app/frontend/dist /home/appuser/frontend/dist

# Set ownership
RUN chown -R appuser:appuser /home/appuser

USER appuser

ENV BROWSER_PATH=/usr/bin/chromium \
    BROWSER_HEADLESS=true \
    PORT=8080 \
    ENV=production

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/usr/local/bin/ethiyo"]

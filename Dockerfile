# ==============================================================================
# Multi-Stage Build: FlareHouse Platform Tooling
# ==============================================================================

# Stage 1: Build React 19 Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Gateway Binary
FROM golang:1.24-alpine AS backend-builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /build/flarehouse-gateway ./cmd/gateway

# Stage 3: Minimal, Hardened Runtime Container
FROM alpine:3.20 AS runner
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -g 10001 appgroup \
    && adduser -u 10001 -G appgroup -s /sbin/nologin -D appuser

# Copy Go binary and static frontend assets
COPY --from=backend-builder --chown=appuser:appgroup /build/flarehouse-gateway /app/flarehouse-gateway
COPY --from=frontend-builder --chown=appuser:appgroup /web/dist /app/web/dist

ENV PORT=8080 \
    STATIC_DIR=/app/web/dist

EXPOSE 8080

USER 10001:10001

ENTRYPOINT ["/app/flarehouse-gateway"]

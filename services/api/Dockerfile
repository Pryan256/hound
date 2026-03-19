# ── Stage 0: Build Link widget ────────────────────────────────────────────────
# Build context must be the repo root (set context: . in docker-compose.yml)
FROM node:20-alpine AS link-builder

WORKDIR /link

# Install dependencies first (better layer caching)
COPY packages/link/package.json ./
RUN npm install

# Copy source and build the hosted page bundle
COPY packages/link/ .
RUN npm run build:hosted

# ── Stage 1: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY services/api/go.mod services/api/go.sum ./
RUN go mod download

COPY services/api/ .

# Copy the compiled Link widget into the static dir before building
COPY --from=link-builder /link/dist-hosted ./static/link/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/api ./cmd/main.go

# ── Stage 2: Final image ──────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Binary
COPY --from=builder /app/bin/api .

# Static assets (hound.js embed script + compiled Link widget)
COPY --from=builder /app/static ./static/

# Database migrations
COPY --from=builder /app/migrations ./migrations/

EXPOSE 8080

CMD ["./api"]

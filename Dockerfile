# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend-builder
WORKDIR /src/frontend

RUN corepack enable && corepack prepare pnpm@9 --activate

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/src ./src
COPY frontend/static ./static
COPY frontend/app.html ./
COPY frontend/components.json ./
COPY frontend/orval.config.ts ./
COPY frontend/svelte.config.js ./
COPY frontend/tsconfig.json ./
COPY frontend/vite.config.ts ./
RUN pnpm build

FROM golang:1.24.5-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /src/frontend/build ./frontend/build

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orvo ./cmd/orvo

FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app

COPY --from=builder /out/orvo ./orvo
RUN chown -R app:app /app

ENV APP_ENVIRONMENT=production
ENV APP_APP_PORT=8080

EXPOSE 8080

USER app
ENTRYPOINT ["./orvo"]

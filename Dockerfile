FROM node:22-alpine AS web-build

WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.22-alpine AS go-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist ./internal/api/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/tspeek ./cmd/server

FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser

WORKDIR /app
COPY --from=go-build /out/tspeek /app/tspeek

USER appuser
EXPOSE 8080

ENV TSPEEK_CONFIG=/config/config.yaml

ENTRYPOINT ["/app/tspeek"]

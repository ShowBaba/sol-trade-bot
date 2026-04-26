# Build Go binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /build
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /solbot ./cmd/solbot

# Build React UI
FROM node:20-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci 2>/dev/null || npm install
COPY web/ ./
RUN npm run build

# Final image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /solbot .
COPY --from=web-builder /web/dist ./web
ENV PORT=8080
ENV STATIC_DIR=/app/web
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/solbot"]

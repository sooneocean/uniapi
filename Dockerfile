# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -o uniapi ./cmd/uniapi

# Stage 3: Final image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/uniapi /usr/local/bin/uniapi
EXPOSE 9000
VOLUME /data
ENV UNIAPI_DATA_DIR=/data
ENTRYPOINT ["uniapi"]

FROM node:22-alpine AS frontend
WORKDIR /build/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY backend/ ./backend/
RUN CGO_ENABLED=0 go build -o /out/crm-server ./backend/cmd/server && \
    CGO_ENABLED=0 go build -o /out/crm-migrate ./backend/cmd/migrate && \
    CGO_ENABLED=0 go build -o /out/crm-seed ./backend/cmd/seed

FROM alpine:3.21
WORKDIR /app
COPY --from=backend /out/crm-server /out/crm-migrate /out/crm-seed ./
COPY --from=frontend /build/frontend/dist ./frontend/dist
RUN adduser -D -u 10001 crm && mkdir /app/uploads && chown crm:crm /app/uploads
USER crm
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s CMD wget -q -O /dev/null http://127.0.0.1:8000/api/health || exit 1
CMD ["/bin/sh", "-c", "./crm-migrate && exec ./crm-server"]

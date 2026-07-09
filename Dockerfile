FROM node:24-alpine AS web
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM golang:1.24-alpine AS api
WORKDIR /src
COPY backend-go/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/panel-api ./cmd/panel-api

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates docker-cli
ENV PANEL_WEB_ROOT=/app/dist
COPY --from=web /app/dist ./dist
COPY --from=api /out/panel-api ./panel-api
EXPOSE 16824
CMD ["./panel-api"]

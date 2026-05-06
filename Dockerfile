ARG GO_VERSION=1.26.1
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:${GO_VERSION}-alpine AS builder
RUN apk add --no-cache ca-certificates git
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /src/web/dist ./cmd/ctf-agent/web_dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ctf-agent ./cmd/ctf-agent

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata docker-cli wget

WORKDIR /app
COPY --from=builder /out/ctf-agent /usr/local/bin/ctf-agent

RUN mkdir -p /data /uploads /logs

ENV CTF_AGENT_ADDR=:4399 \
    CTF_AGENT_CONFIG=/data/config.yaml \
    CTF_AGENT_DB=/data/ctf-agent.db

EXPOSE 4399
VOLUME ["/data", "/uploads", "/logs"]

ENTRYPOINT ["ctf-agent"]
CMD ["server", "--addr", ":4399", "--config", "/data/config.yaml", "--db", "/data/ctf-agent.db"]

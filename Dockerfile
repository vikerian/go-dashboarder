# STAGE 1: Builder
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Goproxy pro rychlejší build v našich končinách
ENV GOPROXY=https://proxy.golang.org,direct

# Cache závislostí
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Argument, který určí, co budeme kompilovat (např. cmd/raw-input)
ARG MODULE_PATH
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/service ./${MODULE_PATH}

# STAGE 2: Runner
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /app/service .
# Kopírujeme i složku s configy, aby měly kontejnery k čemu sahat
COPY configs/ ./configs/

CMD ["./service"]

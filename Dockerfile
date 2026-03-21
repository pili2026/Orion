# ==========================================
# 階段一: Builder
# ==========================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 複製依賴檔並下載
COPY go.mod go.sum ./
RUN go mod download

# 複製原始碼
COPY . .

# 編譯所有執行檔
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/migrate ./cmd/migrate/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/etl-meta ./cmd/etl-meta/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/etl-telemetry ./cmd/etl-telemetry/main.go

# ==========================================
# 階段二: Final
# ==========================================
FROM alpine:latest

# 安裝憑證與時區資料 (你的 compose 有設定 TZ: Asia/Taipei，這裡必須裝 tzdata)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 從 builder 複製編譯好的執行檔與資料庫遷移檔
COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/migrations /app/migrations

# 對外開放 8000 Port
EXPOSE 8000

# 執行 Orion API Server
CMD ["/app/bin/server"]
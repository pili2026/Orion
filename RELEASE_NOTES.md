## v0.2.0 Release Notes

### Background

本版本以「MQTT 即時遙測收集」為主軸，在 v0.1.0 的 ETL 歷史資料匯入基礎上，補上了 Device 管理 API、Auth 中介層，以及完整的 MQTT Ingest 服務（含 Dead-Letter Queue 重試機制），使系統具備從 Edge Gateway 到 TimescaleDB 的端對端即時資料流。

---

### New Features / Changes

#### Device 模組（全新）
- **Model** (`internal/model/device.go`)：新增 `DeviceType` lookup table 常數（CI/SE/SF/ST/SP/SR/SO）。
- **DTO** (`internal/dto/device.go`)：`UpdateDeviceRequest`（含可選 `display_name`）及 `DeviceResponse`。
- **Repository** (`internal/repository/device.go`)：`DeviceRepository` 介面 + GORM 實作，含 `FindByID`、`Update`。
- **Service** (`internal/service/device.go`)：`DeviceService`，封裝 update 邏輯並做 not-found 判斷。
- **Handler** (`internal/handler/device.go`)：`PATCH /api/v1/devices/:id`，允許前端更新裝置顯示名稱。

#### MQTT 即時收集（全新）
- **DTO** (`internal/dto/mqtt.go`)：`TelemetryPayload` / `TelemetryReading`，定義 Edge 上傳格式（JSON，含 Unix ms timestamp、多讀數）；`NormalizeIntField` 將舊式 "-1" 正規化為 `nil`。
- **MQTTIngestService** (`internal/service/mqtt_ingest.go`)：
  - 以 MQTT `username` 為索引解析 Gateway → Device / PointAssignment UUID（in-memory read-write cache，RWMutex 保護）。
  - 依 DeviceType 分派至對應 hypertable（`InsertMeter` / `InsertInverter` / `InsertFlowMeter` / `InsertSensor`）。
  - 記錄 `received_at`（Orion 收到時間），與裝置端 `ts` 分開儲存，方便延遲分析。
  - **Dead-Letter Queue**：寫入失敗的單筆 reading 進入 buffered channel（1024），背景 goroutine 以指數退避（2 s → 4 s → 8 s）最多重試 3 次後丟棄並記錄 Error log；payload 層級錯誤（JSON 解析失敗）直接回傳 error，不進 DLQ。
  - `Start(ctx)` / `Stop()` 支援優雅關閉。

#### Auth 中介層（全新）
- **`internal/middleware/auth.go`**：`Auth(Authenticator)` 解析 `Authorization: Bearer <token>`，驗證後將 `*Identity`（UserID / Roles / Permissions）存入 Gin context。
- **`internal/middleware/permissions.go`**：權限常數集中定義。
- **`internal/middleware/stub_auth.go`**：`StubAuthenticator`，供測試環境注入固定 Identity，無需真實 JWT。
- Router 已於各 API 群組掛載 `Auth` middleware；現有 handler 測試更新為帶入合法 Bearer token。

#### 資料庫 Migration
- **Migration 004** (`004_mqtt_ingest`)：
  - 所有 telemetry hypertable 新增 `received_at TIMESTAMPTZ`（nullable，歷史 ETL 資料為 NULL）。
  - `telemetry_inverters.error` / `.alert` 欄位從 `TEXT` 轉型為 `INTEGER`，`-1` 正規化為 `NULL`（需先解壓縮 chunks 再 ALTER TYPE，migration 已處理 TimescaleDB 壓縮限制）。
- **Migration 005** (`005_add_device_display_name`)：
  - `devices` 表新增可選 `display_name VARCHAR(100)`，NULL 表示未設定，前端應 fallback 至 `func_tag`。

#### 共用套件與設定
- **`pkg/apperr/errors.go`**：新增 `ErrNotFound` sentinel error，各層 handler 使用 `errors.Is` 對應 HTTP 404。
- **`internal/config/config.go`**：`config.Init()` 自動載入 `.env`（使用 godotenv），找不到時回退至系統環境變數。
- **`.air.toml`**：開發熱重載設定（Air）。
- **`Dockerfile`**：多階段 build，最終 image 基於 `gcr.io/distroless/static`。

#### 測試
- 新增 Handler 單元測試：`gateway_test.go`、`site_test.go`、`telemetry_test.go`、`router_test.go`。
- `testutil_test.go`：共用測試 helper（`newTestRouter`、`mustMarshal` 等）。
- `middleware/auth_test.go`：Auth / RequirePermission middleware 測試。

#### 腳本
- `scripts/mqtt_test_pub.sh`：手動發布測試 MQTT 訊息，驗證 Ingest 流程。
- `scripts/test_auth.sh`：驗證 API 的 Auth header 行為（401 / 403 / 200）。
- `scripts/etl.sh`：一鍵執行 etl-meta → etl-telemetry 的完整 ETL 流程。

#### 文件
- `docs/db/er_diagram/database_architecture_specification.md`：資料庫架構規格與 ER 圖說明。

---

### Files Added / Modified

#### New（v0.1.0 之後新增）

| 檔案 | 說明 |
|------|------|
| `internal/model/device.go` | DeviceType 常數擴充 |
| `internal/dto/device.go` | Device DTO |
| `internal/dto/mqtt.go` | MQTT payload DTO + NormalizeIntField |
| `internal/repository/device.go` | DeviceRepository 介面與實作 |
| `internal/service/device.go` | DeviceService |
| `internal/service/mqtt_ingest.go` | MQTTIngestService（cache + DLQ） |
| `internal/handler/device.go` | PATCH /api/v1/devices/:id |
| `internal/middleware/auth.go` | Auth middleware + Identity + RequirePermission |
| `internal/middleware/auth_test.go` | Auth middleware 測試 |
| `internal/middleware/permissions.go` | 權限常數 |
| `internal/middleware/stub_auth.go` | 測試用 StubAuthenticator |
| `internal/config/config.go` | dotenv 設定載入 |
| `pkg/apperr/errors.go` | 共用 sentinel errors |
| `migrations/004_mqtt ingest.up.sql` | received_at + inverter type 轉換 |
| `migrations/004_mqtt ingest.down.sql` | Migration 004 rollback |
| `migrations/005_add_device_display_name.up.sql` | devices.display_name |
| `migrations/005_add_device_display_name.down.sql` | Migration 005 rollback |
| `scripts/mqtt_test_pub.sh` | MQTT 測試發布腳本 |
| `scripts/test_auth.sh` | Auth 流程測試腳本 |
| `scripts/etl.sh` | ETL 一鍵執行腳本 |
| `Dockerfile` | 容器 build |
| `.air.toml` | 開發熱重載設定 |
| `docs/db/README.md` | DB 文件索引 |
| `docs/db/er_diagram/database_architecture_specification.md` | 資料庫架構規格 |

#### Modified（v0.1.0 既有、本版本有明顯異動）

| 檔案 | 異動摘要 |
|------|----------|
| `internal/handler/router.go` | 掛載 Auth middleware；新增 Device 路由 |
| `internal/handler/gateway.go` | 原 `gateway_handler.go` 重新命名；整合 Auth |
| `internal/handler/site.go` | 原 `site_handler.go` 重新命名；整合 Auth |
| `internal/handler/telemetry.go` | 原 `telemetry_handler.go` 重新命名；整合 Auth |
| `internal/handler/gateway_test.go` | 新增 / 更新測試（帶 Bearer token） |
| `internal/handler/site_test.go` | 新增 / 更新測試 |
| `internal/handler/telemetry_test.go` | 新增 / 更新測試 |
| `internal/handler/router_test.go` | Router 整合測試 |
| `internal/handler/testutil_test.go` | 測試共用 helper |
| `internal/service/mqtt.go` | 整合 MQTTIngestService 啟動 / 訂閱回呼 |
| `cmd/server/main.go` | 注入 DeviceService、MQTTIngestService；graceful shutdown 加入 Stop() |
| `docker-compose.yml` | 新增 Orion 服務定義與健康檢查依賴 |
| `.env.example` | 新增 MQTT_TLS_SERVER_NAME、ETL 相關環境變數 |

---

### Known Limitations

1. **Auth backend 尚未實作**：目前 `Authenticator` 介面無生產環境實作（JWT / OAuth2 introspection），`StubAuthenticator` 僅供測試使用；正式部署前需補上。
2. **DLQ 為 in-memory**：`MQTTIngestService` 的 DLQ 不持久化；服務重啟後 channel 內等待重試的 readings 將遺失。
3. **Cache 無過期機制**：UUID resolution cache 不設 TTL，Gateway / Device 刪除或更新後需重啟服務才能清除舊快取。
4. **PATCH /api/v1/devices/:id 功能有限**：目前只支援更新 `display_name`，尚未提供完整的 Device CRUD（Create / Delete）。
5. **Zones API 未整合 Auth**：部分 Zone 子路由是否已全面掛載 `RequirePermission` 待確認。
6. **無整合 / E2E 測試**：現有測試為 handler unit tests（以 httptest + StubAuth 執行），缺乏針對 TimescaleDB hypertable 的整合測試。
7. **Migration 004 檔名含空格**：`004_mqtt ingest.up.sql` 檔名含空格，在某些 CI 腳本中可能需要跳脫處理。

---

### Commit Message

```
chore(release): tag v0.2.0 — MQTT real-time ingest, Device API, Auth middleware

- Add MQTTIngestService with in-memory UUID cache and DLQ retry (3×, exp backoff)
- Add Device CRUD partial (PATCH display_name) with repository/service/handler
- Add Auth middleware layer (Bearer token, Identity, RequirePermission, StubAuth)
- Add migrations 004 (received_at, inverter type fix) and 005 (device display_name)
- Add pkg/apperr sentinel errors and internal/config dotenv loader
- Add Dockerfile, .air.toml, etl.sh, mqtt_test_pub.sh, test_auth.sh
- Add handler unit tests with StubAuthenticator
- Add DB architecture docs
```

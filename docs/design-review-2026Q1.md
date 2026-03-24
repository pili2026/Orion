# Orion 設計審查 2026 Q1

> 審查日期：2026-03-24
> 分支：`claude/review-legacy-schema-PE4UT`

---

## 1. DB Schema 現況

### 1.1 所有 Table 一覽

#### 字典層（Dictionary）

| Table | 主要欄位 | 設計意圖 |
|-------|----------|----------|
| `device_types` | `code` (PK), `description`, `category` (Gateway/Device/Sensor) | 設備類型碼的查找表；ETL seeder 啟動時填充，其他 FK 均引用此表 |

---

#### 中繼資料層（Metadata）

| Table | 主要欄位 | FK | 設計意圖 |
|-------|----------|----|----------|
| `sites` | `id` (PK), `utility_id` (UNIQUE), `name_cn`, `site_code` (UNIQUE) | — | 對應舊版「場域代號」(e.g. `04828405156`) |
| `zones` | `id` (PK), `site_id`, `zone_name`, `display_order` | `site_id → sites.id` | 人類定義的邏輯分組；UNIQUE(site_id, zone_name) |
| `gateways` | `id` (PK), `site_id`, `serial_no` (UNIQUE), `mac` (UNIQUE), `model`, `display_name`, `status`, `network_status`, `ssh_port`, `mqtt_username`, `last_seen_at`, PKI 欄位群 | `site_id → sites.id` | 對應舊版「loop」(e.g. `000gw`, `100gw`)，一台實體 edge 電腦 |
| `devices` | `id` (PK), `gateway_id`, `zone_id`, `device_type_code`, `func_tag`, `display_name`, `device_code` | `gateway_id → gateways.id`，`zone_id → zones.id`（nullable），`device_type_code → device_types.code` | 對應舊版腳位層級的設備（e.g. `020sr` 中的 slave+pin） |
| `physical_points` | `id` (PK), `device_id`, `port_index`；UNIQUE(device_id, port_index) | `device_id → devices.id` | 設備的實體腳位，對應舊版 table name 的 pin 欄位 |
| `point_assignments` | `id` (PK), `point_id`, `zone_id`, `sensor_type_code`, `func_tag`, `sensor_name`, `unit`, `metadata` (JSONB), `active_from`, `active_to` | `point_id → physical_points.id`，`zone_id → zones.id`（nullable），`sensor_type_code → device_types.code` | 感測器的邏輯配置，支援時間窗口（active_from/active_to），用於 `telemetry_sensors` 的 FK |

---

#### ETL 輔助層

| Table | 主要欄位 | 設計意圖 |
|-------|----------|----------|
| `etl_table_map` | `old_table_name` (PK), `utility_id`, `device_type`, `device_id`（nullable UUID）, `assignment_id`（nullable UUID） | 舊版 table name → 新版 UUID 的對照表；非感測器填 device_id，感測器填 assignment_id |
| `etl_checkpoints` | `old_table_name` (PK), `last_id` (TIMESTAMPTZ), `status`, `rows_migrated`, `started_at`, `finished_at`, `error_msg` | 每張 table 的遷移進度記錄，支援中斷後恢復 |

---

#### 遙測層（TimescaleDB Hypertables）

| Table | 時間分區欄位 | device FK | 適用設備 |
|-------|-------------|-----------|----------|
| `telemetry_meters` | `ts` | `device_id` | SE（電錶） |
| `telemetry_inverters` | `ts` | `device_id` | CI（變流器） |
| `telemetry_flow_meters` | `ts` | `device_id` | SF（流量計） |
| `telemetry_sensors` | `ts` | `assignment_id` | ST/SP/SR/SO（各類感測器） |

- Chunk interval：1 week
- Compression policy：30 天後啟用（約節省 90% 空間）
- Migration 004 起增加 `received_at` 欄位（MQTT 即時資料用，歷史 ETL 資料為 NULL）

---

#### PKI 層（Migration 006、007）

| Table | 主要欄位 | 設計意圖 |
|-------|----------|----------|
| `pki_ca` | `id`, `cert_pem`, `key_pem`, `expires_at`, `singleton` (BOOL, UNIQUE) | 根 CA 單例；`UNIQUE(singleton)` 防止多副本競爭插入 |
| `revoked_cert_serials` | `id`, `gateway_id`, `cert_serial`, `revoked_at`, `reason` | Gateway 憑證撤銷稽核記錄；預留 CRL/OCSP 整合用 |

Gateway 的 cert_status 狀態機：`etl_synced → cert_issued → mqtt_pending → mqtt_connected`

---

### 1.2 FK 關係圖（文字版）

```
sites
 ├─ zones (site_id)
 │    ├─ devices (zone_id)          ← nullable，ETL 填 default zone
 │    └─ point_assignments (zone_id) ← nullable，ETL 填 default zone
 └─ gateways (site_id)
      └─ devices (gateway_id)
           ├─ physical_points (device_id)
           │    └─ point_assignments (point_id)
           └─ [telemetry_meters | telemetry_inverters | telemetry_flow_meters]
                 ← 以 device_id 關聯（無 FK constraint，僅應用層保證）

point_assignments
  └─ telemetry_sensors (assignment_id)
       ← 以 assignment_id 關聯（無 FK constraint，僅應用層保證）
```

> **注意**：遙測 hypertable 的 `device_id`/`assignment_id` 欄位均**無資料庫層級 FK constraint**（TimescaleDB 壓縮與 FK 不相容），一致性由應用層保證。

---

## 2. ETL 映射分析

### 2.1 舊版 Loop → 新版 Gateway

**程式碼位置**：`internal/etl/parser.go`（解析）、`internal/etl/seeder.go`（建立）

**映射邏輯**：

1. `parser.go` 解析舊版 table name，格式為 `{utility_id}_{loop}{slave_hex}{pin}{type}`
   - 例：`04828405156_1f0sf` → `utility_id="04828405156"`, `loop=1`, `slave_hex="f"`, `pin=0`, `type="sf"`
   - **注意**：目前 parser 假設 suffix 固定 5 個字元（1 loop + 1 slave + 1 pin + 2 type）；多位數 slave 會解析失敗

2. `GatewayKey()` 產生 Gateway 的去重 key：`"{utility_id}-loop{loop}"`
   - 例：`04828405156-loop0`、`04828405156-loop1`

3. `seeder.go` 用 `serial_no = GatewayKey` 做 `FirstOrCreate`
   - 舊版的 `000gw` loop 編號（000, 100, ...）在 seeder 中對應的是 `loop` 整數（0, 1, ...），**不是** 原始的三位數字串
   - ETL 建立的 Gateway 皆為佔位記錄：`mac = GatewayKey`（placeholder），`model = "legacy"`

**問題**：舊版 loop 編號如 `000`、`100` 會被 parser 解析為整數 `0` 和 `1`，但 `000gw` 的格式實際上代表的是「第 0 個 loop 的 gateway」，這個語意**未完整保存**（原始三位字串 000/100 被丟棄）。

---

### 2.2 Device / PhysicalPoint 的建立邏輯

**程式碼位置**：`internal/etl/seeder.go:214-265`（`upsertDeviceAndMapping`）

1. `device_code = "{loop}{slave_hex}{pin}"`，例如 `1f0`，在 gateway 內唯一
2. `func_tag = "{TYPE}_{device_code}"`，例如 `SF_1f0`
3. `zone_id` 填入 **default zone** 的 UUID（見 2.3）
4. 非感測器（SE/CI/SF）：只建 Device，`etl_table_map.device_id` 填 Device UUID
5. 感測器（ST/SP/SR/SO）：另外建 `PhysicalPoint`（port_index = pin）及 `PointAssignment`，`etl_table_map.assignment_id` 填 PointAssignment UUID
6. GW 類型 table 直接跳過（`seeder.go:146-150`），不建立任何 Device 記錄

---

### 2.3 Zone 目前的建立方式及 GatewayID 狀況

**程式碼位置**：`internal/etl/seeder.go:114-128`

```go
// seeder.go 第 116-128 行
zone := &model.Zone{
    SiteID:       site.ID,
    ZoneName:     "default",
    DisplayOrder: 0,
}
s.dstDB.Where(model.Zone{SiteID: site.ID, ZoneName: "default"}).
    FirstOrCreate(zone)
```

**現況**：
- ETL **只為每個 Site 建一個 `zone_name = "default"` 的 Zone**
- 該 Zone **沒有 GatewayID 欄位**（`zones` table schema 本身也沒有此欄位）
- 所有 Device 的 `zone_id` 均指向同一個 default Zone
- PointAssignment 的 `zone_id` 也繼承自 Device 的 ZoneID（default Zone）
- **Loop → Zone 的映射關係完全遺失**

---

## 3. 設計缺口

### 3.1 Zone 缺少 GatewayID 欄位

**問題**：根據設計意圖，Zone 是「人類給 Gateway 取的有意義名稱」，應該與 Gateway 存在 1:1 關係。但目前：

| 位置 | 現況 | 問題 |
|------|------|------|
| `migrations/001_init_schema.up.sql:32-43` | `zones` table 無 `gateway_id` 欄位 | 無法表達 Zone ↔ Gateway 的 1:1 關係 |
| `internal/model/site.go:20-29` | `Zone` struct 無 `GatewayID` 欄位 | 應用層無法使用此關係 |
| `internal/dto/site.go` | `ZoneResponse`/`CreateZoneRequest`/`UpdateZoneRequest` 均無 gateway_id | API 無法傳遞此資訊 |
| `internal/handler/site.go` | CreateZone/UpdateZone handler 無法接收 gateway_id | — |
| `internal/service/site.go` | ZoneService 無法處理 gateway_id | — |
| `internal/repository/site.go` | ZoneRepository 無法查詢 gateway_id | — |

**需要新增 migration**（尚不存在）：
```sql
ALTER TABLE zones ADD COLUMN gateway_id UUID REFERENCES gateways(id);
CREATE UNIQUE INDEX idx_zones_gateway_id ON zones(gateway_id) WHERE deleted_at IS NULL;
```

---

### 3.2 ETL 建立 default Zone 的影響範圍

**具體位置**：`internal/etl/seeder.go:114-128`（Zone 建立）及第 226 行（`ZoneID: defaultZone.ID`）

**影響鏈**：
1. `seeder.go` 為每個 site 建立一個 `default` Zone（無 gateway_id）
2. 所有 Device 的 `zone_id` 均指向此 default Zone
3. 所有感測器的 `PointAssignment.zone_id` 也指向 default Zone（繼承自 device.ZoneID）
4. **後果**：ListZones API 回傳的 Zone 無法對應到任何 Gateway，操作人員無法判斷哪個 Zone 對應哪台 edge 電腦

**需要處理的工作**：
- ETL 完成後，操作人員需手動建立有意義的 Zone（例如「一樓機房」），並在建立時指定 `gateway_id`
- 目前 `CreateZoneRequest` **不接受** `gateway_id`，需要先修改 API
- 修改後，操作人員才能把各 Gateway 的 Device 從 default Zone 搬移到正確 Zone

---

### 3.3 Device.ZoneID 目前 ETL 的填充方式

**具體位置**：`internal/etl/seeder.go:226`

```go
device := &model.Device{
    GatewayID:      gw.ID,
    ZoneID:         defaultZone.ID, // placeholder — reassign zones manually after seeding
    ...
}
```

- 所有 Device 的 `zone_id` 在 ETL 後均指向 **per-site 的 default Zone**
- 這是刻意的佔位設計（程式碼有 comment 說明），但目前沒有任何工具或 API flow 協助後續重新指派
- `devices.zone_id` 在 DDL 層是 nullable（`REFERENCES zones(id)` 無 NOT NULL），但 model 層的 GORM tag 及 DTO 均未反映此可空性

---

## 4. API 影響範圍

### 4.1 Zone 相關 API Endpoint

| Method | Path | 說明 | 是否需要調整 |
|--------|------|------|-------------|
| `GET` | `/api/v1/sites/:id/zones` | 列出 Site 下所有 Zone | ⚠️ Response 應加入 `gateway_id` |
| `POST` | `/api/v1/sites/:id/zones` | 建立 Zone（需 PermSiteWrite） | ✅ **必須**加入 `gateway_id` 欄位（UNIQUE 約束） |
| `PATCH` | `/api/v1/sites/:id/zones/:zone_id` | 更新 Zone（需 PermSiteWrite） | ⚠️ 可選加入 `gateway_id` 更新能力 |
| `DELETE` | `/api/v1/sites/:id/zones/:zone_id` | 刪除 Zone（需 PermSiteDelete） | 不需調整 |

### 4.2 間接受影響的 API

| Method | Path | 說明 | 影響 |
|--------|------|------|------|
| `GET` | `/api/v1/gateways` | 列出 Gateway | 理想上應能反向查詢對應的 Zone |
| `GET` | `/api/v1/gateways/:id` | 取得單一 Gateway | 同上 |
| `PATCH` | `/api/v1/devices/:id` | 更新 Device | 目前只能改 display_name，**不能重新指派 zone_id** |

### 4.3 目前不存在但需要的 API

| Method | Path | 說明 |
|--------|------|------|
| `PATCH` | `/api/v1/devices/:id` (擴充) | 支援更新 `zone_id`，讓操作人員能把 Device 從 default Zone 搬到有意義的 Zone |

---

## 5. 建議修改清單（按優先序）

---

### P0：新增 Migration — Zone 加入 GatewayID

**檔案**：`migrations/008_zone_gateway_id.up.sql`（新建）

**目前狀況**：`zones` table 無 `gateway_id` 欄位，Zone ↔ Gateway 關係無法在資料層表達。

**建議修改**：
```sql
ALTER TABLE zones ADD COLUMN gateway_id UUID REFERENCES gateways(id);
-- 一個 Gateway 最多對應一個 Zone（1:1），用 partial unique index 允許 NULL
CREATE UNIQUE INDEX idx_zones_gateway_id ON zones(gateway_id) WHERE deleted_at IS NULL;
```

**down migration**（`migrations/008_zone_gateway_id.down.sql`）：
```sql
DROP INDEX IF EXISTS idx_zones_gateway_id;
ALTER TABLE zones DROP COLUMN IF EXISTS gateway_id;
```

---

### P0：更新 Zone Model

**檔案**：`internal/model/site.go:20-29`

**目前狀況**：
```go
type Zone struct {
    BaseModel
    SiteID       uuid.UUID `gorm:"type:uuid;not null;index"  json:"site_id"`
    ZoneName     string    `gorm:"type:varchar(100);not null" json:"zone_name"`
    DisplayOrder int       `gorm:"type:int;not null;default:0" json:"display_order"`
    Site    *Site    `gorm:"foreignKey:SiteID"  json:"site,omitempty"`
    Devices []Device `gorm:"foreignKey:ZoneID"  json:"devices,omitempty"`
}
```

**建議修改**：加入 `GatewayID` 欄位（nullable，1:1）：
```go
type Zone struct {
    BaseModel
    SiteID       uuid.UUID  `gorm:"type:uuid;not null;index"               json:"site_id"`
    GatewayID    *uuid.UUID `gorm:"type:uuid;uniqueIndex;default:null"      json:"gateway_id,omitempty"`
    ZoneName     string     `gorm:"type:varchar(100);not null"              json:"zone_name"`
    DisplayOrder int        `gorm:"type:int;not null;default:0"             json:"display_order"`
    Site    *Site    `gorm:"foreignKey:SiteID"   json:"site,omitempty"`
    Gateway *Gateway `gorm:"foreignKey:GatewayID" json:"gateway,omitempty"`
    Devices []Device `gorm:"foreignKey:ZoneID"   json:"devices,omitempty"`
}
```

---

### P0：更新 Zone DTO

**檔案**：`internal/dto/site.go`

**目前狀況**：`ZoneResponse`、`CreateZoneRequest`、`UpdateZoneRequest` 均無 `GatewayID`。

**建議修改**：
```go
type ZoneResponse struct {
    ID           uuid.UUID  `json:"id"`
    SiteID       uuid.UUID  `json:"site_id"`
    GatewayID    *uuid.UUID `json:"gateway_id,omitempty"`  // 新增
    ZoneName     string     `json:"zone_name"`
    DisplayOrder int        `json:"display_order"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateZoneRequest struct {
    GatewayID    *string `json:"gateway_id"`       // 新增，optional（default zone 可無）
    ZoneName     string  `json:"zone_name" binding:"required"`
    DisplayOrder int     `json:"display_order"`
}

type UpdateZoneRequest struct {
    GatewayID    *string `json:"gateway_id"`        // 新增，optional
    ZoneName     *string `json:"zone_name"`
    DisplayOrder *int    `json:"display_order"`
}
```

---

### P1：更新 Zone Service 與 Repository

**檔案**：`internal/service/site.go`、`internal/repository/site.go`

**目前狀況**：Create/Update 邏輯均不處理 `gateway_id`。

**建議修改**：
- `Create`：若 request 帶有 `gateway_id`，驗證該 Gateway 屬於同一個 Site，再寫入 Zone
- `Update`：同上，支援更新 `gateway_id`
- `Repository.Create`/`Update`：無需變更，GORM 會自動處理新欄位

---

### P1：更新 Device DTO 及 Handler — 支援 ZoneID 重新指派

**檔案**：`internal/dto/device.go`、`internal/handler/device.go`、`internal/service/device.go`

**目前狀況**：`UpdateDeviceRequest` 只有 `DisplayName`，無法重新指派 `ZoneID`。

**建議修改**：
```go
type UpdateDeviceRequest struct {
    DisplayName *string `json:"display_name"`
    ZoneID      *string `json:"zone_id"`   // 新增，允許操作人員把 Device 從 default Zone 移走
}
```

- Service 層需驗證新 ZoneID 與 Device 的 Gateway 屬於同一個 Site

---

### P2：ETL Seeder — 改為每個 Gateway 建一個 Default Zone

**檔案**：`internal/etl/seeder.go:114-128`

**目前狀況**：每個 Site 只建一個 `default` Zone，所有 Gateway 共用。

**建議修改**：改為每個 Gateway 建一個對應的 default Zone，並填入 `gateway_id`：
```go
for _, gw := range gatewayMap {
    zone := &model.Zone{
        SiteID:       gw.SiteID,
        GatewayID:    &gw.ID,  // 建立 Zone ↔ Gateway 1:1 關係
        ZoneName:     gw.SerialNo, // 或 "default-{serial_no}"
        DisplayOrder: 0,
    }
    s.dstDB.Where(model.Zone{GatewayID: &gw.ID}).FirstOrCreate(zone)
    // 將此 zone 指派給該 gw 下的所有 devices
}
```

- 此改動會破壞現有 ETL 產生的 default Zone 結構，需搭配資料遷移腳本

---

### P3：補上 GatewayResponse 中的 Zone 資訊

**檔案**：`internal/dto/gateway.go`、`internal/handler/gateway.go`

**目前狀況**：Gateway API response 沒有對應 Zone 的資訊，操作人員無法直接知道某台 Gateway 被指派到哪個 Zone。

**建議修改**：在 `GatewayResponse` 加入 `ZoneID *string` 和 `ZoneName *string`（從 zones 表 JOIN 查詢）。

---

## 附錄：檔案索引

| 類別 | 路徑 |
|------|------|
| Migration（現有） | `migrations/001_init_schema.up.sql` ～ `007_pki_race_and_revocations.up.sql` |
| Zone Model | `internal/model/site.go` |
| Zone DTO | `internal/dto/site.go` |
| Zone Handler | `internal/handler/site.go` |
| Zone Service | `internal/service/site.go` |
| Zone Repository | `internal/repository/site.go` |
| Gateway Model | `internal/model/gateway.go` |
| Gateway DTO | `internal/dto/gateway.go` |
| Device Model | `internal/model/device.go` |
| Device DTO | `internal/dto/device.go` |
| Device Handler | `internal/handler/device.go` |
| ETL Parser | `internal/etl/parser.go` |
| ETL Seeder | `internal/etl/seeder.go` |
| ETL Telemetry | `internal/etl/telemetry.go` |
| API Routes | `internal/handler/router.go` |

# Orion / Talos Core Database Architecture Specification - Data Dictionary

## I. System Dictionary Layer

This layer defines the internal enumeration values of the system, providing standardized codes for devices and sensors.

### 1. `device_types` (Device & Sensor Type Dictionary)

| Column        | Type | Constraints & Keys | Description                                    |
| ------------- | ---- | ------------------ | ---------------------------------------------- |
| `code`        | TEXT | **PK**             | Type code (e.g., CI, SE, SR, ST)               |
| `description` | TEXT | NOT NULL           | Full description (e.g., Inverter, Power Meter) |
| `category`    | TEXT | CHECK              | Main category (`Gateway`, `Device`, `Sensor`)  |

---

## II. Metadata Layer (Master Data)

This layer records physical hardware assets and dashboard logical groups, fully supporting Soft Delete (GORM standard).

### 2. `sites` (Facilities / Sites)

| Column       | Type        | Constraints & Keys  | Description                                              |
| ------------ | ----------- | ------------------- | -------------------------------------------------------- |
| `id`         | UUID        | **PK** (Default v4) | Unique identifier for the site                           |
| `utility_id` | TEXT        | **UK**, NOT NULL    | Utility account number (Primary key for external search) |
| `name_cn`    | TEXT        | NOT NULL            | Site display name (e.g., Compeq Plant A)                 |
| `site_code`  | TEXT        | **UK**              | Internal English code (e.g., `compeq_a`)                 |
| `created_at` | TIMESTAMPTZ | Default NOW()       | Creation timestamp                                       |
| `updated_at` | TIMESTAMPTZ | Default NOW()       | Last update timestamp                                    |
| `deleted_at` | TIMESTAMPTZ | INDEX               | GORM soft delete flag                                    |

### 3. `zones` (Dashboard Groups / Zones)

| Column          | Type        | Constraints & Keys    | Description                                      |
| --------------- | ----------- | --------------------- | ------------------------------------------------ |
| `id`            | UUID        | **PK** (Default v4)   | Unique identifier for the zone                   |
| `site_id`       | UUID        | **FK** (-> sites), NN | The site this zone belongs to                    |
| `zone_name`     | TEXT        | NOT NULL              | Dashboard section name (e.g., '1', '2', 'Front') |
| `display_order` | INTEGER     |                       | Sorting weight for frontend UI                   |
| `created_at`    | TIMESTAMPTZ | Default NOW()         | Creation timestamp                               |
| `updated_at`    | TIMESTAMPTZ | Default NOW()         | Last update timestamp                            |
| `deleted_at`    | TIMESTAMPTZ | INDEX                 | GORM soft delete flag                            |

> **Composite Unique Key (UK)**: `(site_id, zone_name)` ensures no duplicate zone names within the same site.

### 4. `gateways` (Edge Computing Gateways)

| Column           | Type        | Constraints & Keys    | Description                           |
| ---------------- | ----------- | --------------------- | ------------------------------------- |
| `id`             | UUID        | **PK** (Default v4)   | Unique identifier for the gateway     |
| `site_id`        | UUID        | **FK** (-> sites), NN | The site where the GW is installed    |
| `serial_no`      | TEXT        | **UK**, NOT NULL      | Hardware serial number                |
| `mac`            | TEXT        | **UK**, NOT NULL      | MAC address                           |
| `model`          | TEXT        | NOT NULL              | Hardware model (e.g., Raspberry Pi 4) |
| `display_name`   | TEXT        | NOT NULL              | Frontend UI display name              |
| `status`         | TEXT        | Default 'offline'     | Operational status                    |
| `network_status` | TEXT        | Default 'offline'     | Network connection status             |
| `ssh_port`       | INTEGER     |                       | Remote SSH port                       |
| `mqtt_username`  | TEXT        | NOT NULL              | Account username for MQTT Broker      |
| `last_seen_at`   | TIMESTAMPTZ |                       | Last heartbeat / online time          |
| `created_at`     | TIMESTAMPTZ | Default NOW()         | Creation timestamp                    |
| `updated_at`     | TIMESTAMPTZ | Default NOW()         | Last update timestamp                 |
| `deleted_at`     | TIMESTAMPTZ | INDEX                 | GORM soft delete flag                 |

### 5. `devices` (Modbus Physical Devices)

| Column             | Type        | Constraints & Keys       | Description                                     |
| ------------------ | ----------- | ------------------------ | ----------------------------------------------- |
| `id`               | UUID        | **PK** (Default v4)      | Unique identifier for the device                |
| `gateway_id`       | UUID        | **FK** (-> gateways), NN | The edge gateway it connects to                 |
| `zone_id`          | UUID        | **FK** (-> zones)        | The dashboard zone it belongs to (UI placement) |
| `device_type_code` | TEXT        | **FK** (-> device_types) | Device type (e.g., CI, SE, SF)                  |
| `func_tag`         | TEXT        |                          | UI function tag (e.g., `CT`, `INVERTER_MAIN`)   |
| `device_code`      | TEXT        | NOT NULL                 | Original Modbus ID/Code (e.g., 010, 020)        |
| `created_at`       | TIMESTAMPTZ | Default NOW()            | Creation timestamp                              |
| `updated_at`       | TIMESTAMPTZ | Default NOW()            | Last update timestamp                           |
| `deleted_at`       | TIMESTAMPTZ | INDEX                    | GORM soft delete flag                           |

> **Composite Unique Key (UK)**: `(gateway_id, device_code)` ensures no duplicate Modbus IDs under the same Gateway.

### 6. `physical_points` (AI/DI Physical Pins/Ports)

| Column       | Type        | Constraints & Keys      | Description                           |
| ------------ | ----------- | ----------------------- | ------------------------------------- |
| `id`         | UUID        | **PK** (Default v4)     | Unique identifier for the pin/port    |
| `device_id`  | UUID        | **FK** (-> devices), NN | The AI/DI Module it belongs to        |
| `port_index` | INTEGER     | NOT NULL                | Physical pin/port number (e.g., 1, 5) |
| `created_at` | TIMESTAMPTZ | Default NOW()           | Creation timestamp                    |
| `updated_at` | TIMESTAMPTZ | Default NOW()           | Last update timestamp                 |
| `deleted_at` | TIMESTAMPTZ | INDEX                   | GORM soft delete flag                 |

> **Composite Unique Key (UK)**: `(device_id, port_index)` ensures no duplicate port indices on the same module.

### 7. `point_assignments` (Sensor / Virtual Point Assignments)

| Column             | Type        | Constraints & Keys          | Description                                                                               |
| ------------------ | ----------- | --------------------------- | ----------------------------------------------------------------------------------------- |
| `id`               | UUID        | **PK** (Default v4)         | Unique assignment ID (Crucial FK for narrow tables)                                       |
| `point_id`         | UUID        | **FK** (-> physical_points) | The physical pin it is bound to                                                           |
| `zone_id`          | UUID        | **FK** (-> zones)           | The dashboard zone it belongs to (UI placement)                                           |
| `sensor_type_code` | TEXT        | **FK** (-> device_types)    | Sensor type (e.g., ST, SP, SR)                                                            |
| `func_tag`         | TEXT        |                             | UI function tag (e.g., `CWP_AUTO`, `Tin`)                                                 |
| `sensor_name`      | TEXT        | NOT NULL                    | Display name (e.g., Cooling Water Pump Auto)                                              |
| `unit`             | TEXT        |                             | Measurement unit (e.g., °C, Pa)                                                           |
| `metadata`         | JSONB       |                             | Extra parameters (e.g., calibration coefficient or status dictionary like `{"0": "Off"}`) |
| `active_from`      | TIMESTAMPTZ | Default NOW(), NN           | Assignment start time                                                                     |
| `active_to`        | TIMESTAMPTZ |                             | Assignment end time (NULL means currently active)                                         |
| `created_at`       | TIMESTAMPTZ | Default NOW()               | Creation timestamp                                                                        |
| `updated_at`       | TIMESTAMPTZ | Default NOW()               | Last update timestamp                                                                     |
| `deleted_at`       | TIMESTAMPTZ | INDEX                       | GORM soft delete flag                                                                     |

---

## III. Telemetry Layer (TimescaleDB)

This layer is responsible for storing massive amounts of historical trajectories uploaded by devices.
**Characteristics**: No `updated_at`/`deleted_at`. Data is append-only (Insert and Select). Partitioned by `ts` (Hypertable).

### 8. `telemetry_meters` (Power Meters - SE)

| Column      | Type        | Description                             |
| ----------- | ----------- | --------------------------------------- |
| `ts`        | TIMESTAMPTZ | **Hypertable Partition Key**, Timestamp |
| `device_id` | UUID        | FK referencing `devices(id)`            |
| `voltage`   | NUMERIC     | Voltage (V)                             |
| `current`   | NUMERIC     | Current (A)                             |
| `kw`        | NUMERIC     | Active Power (kW)                       |
| `kva`       | NUMERIC     | Apparent Power (kVA)                    |
| `kvar`      | NUMERIC     | Reactive Power (kVAR)                   |
| `kwh`       | NUMERIC     | Active Energy (kWh)                     |
| `kvah`      | NUMERIC     | Apparent Energy (kVAh)                  |
| `kvarh`     | NUMERIC     | Reactive Energy (kVARh)                 |
| `current_a` | NUMERIC     | Phase A Current                         |
| `current_b` | NUMERIC     | Phase B Current                         |
| `current_c` | NUMERIC     | Phase C Current                         |
| `pf`        | NUMERIC     | Power Factor                            |
| `status`    | TEXT        | Status code / Remarks                   |

### 9. `telemetry_inverters` (Inverters - CI)

| Column      | Type        | Description                             |
| ----------- | ----------- | --------------------------------------- |
| `ts`        | TIMESTAMPTZ | **Hypertable Partition Key**, Timestamp |
| `device_id` | UUID        | FK referencing `devices(id)`            |
| `voltage`   | NUMERIC     | Voltage (V)                             |
| `current`   | NUMERIC     | Current (A)                             |
| `kw`        | NUMERIC     | Active Power (kW)                       |
| `kwh`       | NUMERIC     | Active Energy (kWh)                     |
| `hz`        | NUMERIC     | Frequency (Hz)                          |
| `error`     | TEXT        | Error Code                              |
| `alert`     | TEXT        | Alert Code                              |
| `invstatus` | TEXT        | Internal Inverter Status Code           |
| `status`    | TEXT        | Status code / Remarks                   |

### 10. `telemetry_flow_meters` (Flowmeters - SF)

| Column           | Type        | Description                             |
| ---------------- | ----------- | --------------------------------------- |
| `ts`             | TIMESTAMPTZ | **Hypertable Partition Key**, Timestamp |
| `device_id`      | UUID        | FK referencing `devices(id)`            |
| `flow`           | NUMERIC     | Instantaneous Flow                      |
| `consumption`    | NUMERIC     | Accumulated Flow (Forward)              |
| `revconsumption` | NUMERIC     | Accumulated Flow (Reverse)              |
| `direction`      | INTEGER     | Flow Direction State                    |
| `status`         | TEXT        | Status code / Remarks                   |

### 11. `telemetry_sensors` (AI/DI Universal Narrow Table - ST, SP, SO, SR)

| Column          | Type        | Description                                                         |
| --------------- | ----------- | ------------------------------------------------------------------- |
| `ts`            | TIMESTAMPTZ | **Hypertable Partition Key**, Timestamp                             |
| `assignment_id` | UUID        | FK referencing `point_assignments(id)` (Defines unit & meaning)     |
| `val`           | NUMERIC     | Numeric value (DI strings like '01'/'00' are converted to 1/0 here) |
| `status`        | TEXT        | Raw string or non-numeric status (e.g., `ctrl:00, bypass:0`)        |

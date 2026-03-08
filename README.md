# Orion

A cloud server for IoT/industrial monitoring, built with Go for high performance and maintainability.

## Tech Stack

| Technology                                                  | Purpose                              |
| ----------------------------------------------------------- | ------------------------------------ |
| [Go 1.24+](https://golang.org/)                             | Primary language                     |
| [Gin](https://github.com/gin-gonic/gin)                     | HTTP web framework                   |
| [GORM](https://gorm.io/)                                    | ORM for metadata CRUD                |
| [pgx](https://github.com/jackc/pgx)                         | PostgreSQL driver (telemetry writes) |
| [paho.mqtt](https://github.com/eclipse/paho.mqtt.golang)    | MQTT client                          |
| [golang-migrate](https://github.com/golang-migrate/migrate) | Database migrations                  |
| [Air](https://github.com/air-verse/air)                     | Hot reload for local development     |

## Project Structure

```
orion/
├── cmd/
│   ├── server/
│   │   └── main.go             # Application entry point (graceful shutdown, DI wiring)
│   ├── migrate/
│   │   └── main.go             # Database migration runner (golang-migrate)
│   ├── etl-meta/
│   │   └── main.go             # Stage 2: migrate metadata from legacy MariaDB
│   └── etl-telemetry/
│       └── main.go             # Stage 3: migrate time-series data (checkpoint/resume)
│
├── internal/
│   ├── config/                 # Environment variable loading
│   ├── database/               # DBManager (GORM + pgxpool dual connection)
│   ├── handler/                # HTTP handlers & MQTT subscribers
│   ├── service/                # Business logic (gateway, site, zone, telemetry, dynsec, mqtt)
│   ├── repository/             # Database access layer (GORM + pgx)
│   ├── model/                  # GORM struct definitions
│   ├── dto/                    # Request & response structs
│   ├── etl/                    # ETL pipeline (parser, seeder, telemetry worker)
│   └── middleware/             # Gin middleware (auth, logging, CORS)
│
├── migrations/                 # SQL migration files (golang-migrate)
│   ├── 001_init_schema          # Core schema (sites, zones, gateways, devices, telemetry)
│   ├── 002_etl_tables           # ETL helpers (etl_table_map, etl_checkpoints)
│   └── 003_hypertables          # TimescaleDB hypertables + compression policies
│
├── pkg/
│   ├── logger/                 # Centralized logging
│   └── errors/                 # Custom error types
│
├── deploy/
│   └── docker/                 # Dockerfile and related configs
│
├── docs/
│   ├── api/                    # Bruno API collection
│   ├── db/                     # Database schema & ER diagram
│   └── mqtt_topics.md          # MQTT topic design
│
├── scripts/
│   ├── mqtt_setup.sh           # Mosquitto Dynamic Security initialisation
│   └── mqtt_add_edge.sh        # Register a new Edge device in Mosquitto
│
├── docker-compose.yml
└── README.md
```

## Architecture

### HTTP Request Flow

```
HTTP Request → Middleware → Handler → Service → Repository → Database
                                         ↓
                                   DynsecService (MQTT $CONTROL)
```

| Layer          | Responsibility                                      |
| -------------- | --------------------------------------------------- |
| **Handler**    | Parse & validate request, return response           |
| **Service**    | Business logic, orchestrate repositories and dynsec |
| **Repository** | All database operations (GORM / pgx)                |
| **Model**      | Database schema definitions                         |
| **DTO**        | API input/output structures (separate from models)  |

### MQTT Architecture

Orion communicates with Edge devices (Talos) via MQTT over TLS (port 8883).

```
Edge (Talos)                        Orion (Cloud)
─────────────────────────────────────────────────
talos/{edge_id}/telemetry  ──────►  Subscribe talos/+/telemetry
talos/{edge_id}/status     ──────►  Subscribe talos/+/status
talos/{edge_id}/event      ──────►  Subscribe talos/+/event
talos/{edge_id}/response   ──────►  Subscribe talos/+/response

talos/{edge_id}/command    ◄──────  Publish talos/{id}/command
talos/{edge_id}/config     ◄──────  Publish talos/{id}/config
talos/{edge_id}/ota        ◄──────  Publish talos/{id}/ota

orion/broadcast/#          ◄──────  Publish orion/broadcast/#
```

See [docs/mqtt_topics.md](docs/mqtt_topics.md) for full topic design and payload formats.

---

## Getting Started

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- PostgreSQL with TimescaleDB extension (provided via Docker)
- Mosquitto 2.0+ with Dynamic Security Plugin (provided via Docker)

### Local Development

**1. Clone the repository**

```bash
git clone https://github.com/yourname/orion.git
cd orion
```

**2. Set up environment variables**

```bash
cp .env.example .env
# Fill in DB credentials, MQTT credentials, and other required values
```

**3. Start infrastructure**

```bash
docker-compose up -d
```

**4. Initialise Mosquitto Dynamic Security** (first time only)

```bash
./scripts/mqtt_setup.sh <admin_password> <orion_mqtt_password>
```

**5. Run database migrations**

```bash
go run cmd/migrate/main.go up
```

**6. Start the server with hot reload**

```bash
air
```

The server will be available at `http://localhost:8080`.

---

## Database Migrations

Migrations are managed with [golang-migrate](https://github.com/golang-migrate/migrate).
SQL files live in `migrations/` and follow the naming convention:

```
{version}_{description}.up.sql    # apply
{version}_{description}.down.sql  # rollback
```

### Commands

```bash
# Apply all pending migrations
go run cmd/migrate/main.go up

# Roll back the last migration
go run cmd/migrate/main.go down

# Show current migration version
go run cmd/migrate/main.go version
```

> **Never edit an existing migration file that has already been applied.**
> Always create a new numbered migration for schema changes.

---

## ETL Pipeline (Legacy MariaDB → TimescaleDB)

The ETL pipeline migrates historical data from the legacy `ima_thing` MariaDB database
into Orion's TimescaleDB hypertables. It runs as two independent CLI commands.

### Stage 2 — Metadata (`etl-meta`)

Scans legacy table names, parses device identifiers, and seeds the Orion metadata tables:
`sites`, `zones`, `gateways`, `devices`, `point_assignments`, and `etl_table_map`.

```bash
go run cmd/etl-meta/main.go
```

This command is **idempotent** — safe to re-run if interrupted.

### Stage 3 — Telemetry (`etl-telemetry`)

Reads `etl_table_map` and migrates time-series rows into the appropriate hypertable.
Supports checkpoint/resume — interrupted runs pick up where they left off.

```bash
# Migrate everything (default: last 1 year)
go run cmd/etl-telemetry/main.go

# Specify a date range
ETL_FROM=2024-01-01 ETL_TO=2024-06-30 go run cmd/etl-telemetry/main.go

# Migrate a specific site only
ETL_UTILITY_IDS=05755a6b1a1 go run cmd/etl-telemetry/main.go

# Combine filters — one site, one month, SE devices only
ETL_UTILITY_IDS=05755a6b1a1 ETL_FROM=2024-01-01 ETL_TO=2024-01-31 ETL_DEVICE_TYPES=SE \
  go run cmd/etl-telemetry/main.go
```

### ETL Environment Variables

| Variable             | Default    | Description                                      |
| -------------------- | ---------- | ------------------------------------------------ |
| `ETL_WORKERS`        | `1`        | Number of concurrent table workers               |
| `ETL_BATCH_SLEEP_MS` | `200`      | Sleep between batches (ms) — rate limiting       |
| `ETL_FROM`           | 1 year ago | Start of date range (`YYYY-MM-DD`)               |
| `ETL_TO`             | today      | End of date range (`YYYY-MM-DD`)                 |
| `ETL_UTILITY_IDS`    | _(all)_    | Comma-separated utility IDs to filter            |
| `ETL_DEVICE_TYPES`   | _(all)_    | Comma-separated device types (`SE`, `CI`, `SF`…) |

> `ETL_WORKERS` and `ETL_BATCH_SLEEP_MS` are stable settings — put them in `.env`.
> The other parameters change per run — pass them on the command line.

---

## API

Interactive API documentation is available via the Bruno collection in `docs/api/`.

Open Bruno → **Open Collection** → select `docs/api/`, then switch to the **local** environment.

### Endpoints

| Method   | Path                               | Description                              |
| -------- | ---------------------------------- | ---------------------------------------- |
| `GET`    | `/health`                          | Liveness probe                           |
| `GET`    | `/api/v1/sites`                    | List all sites                           |
| `POST`   | `/api/v1/sites`                    | Create a new site                        |
| `GET`    | `/api/v1/sites/:id`                | Get a single site                        |
| `PATCH`  | `/api/v1/sites/:id`                | Update site info                         |
| `DELETE` | `/api/v1/sites/:id`                | Soft-delete site                         |
| `GET`    | `/api/v1/sites/:id/latest`         | Site-wide real-time snapshot (all zones) |
| `GET`    | `/api/v1/sites/:id/zones`          | List zones for a site                    |
| `POST`   | `/api/v1/sites/:id/zones`          | Create a zone                            |
| `PATCH`  | `/api/v1/sites/:id/zones/:zone_id` | Update zone name / display order         |
| `DELETE` | `/api/v1/sites/:id/zones/:zone_id` | Soft-delete zone                         |
| `POST`   | `/api/v1/gateways`                 | Register gateway + provision MQTT        |
| `GET`    | `/api/v1/gateways`                 | List all gateways                        |
| `GET`    | `/api/v1/gateways/:id`             | Get a single gateway                     |
| `PATCH`  | `/api/v1/gateways/:id`             | Update gateway info                      |
| `DELETE` | `/api/v1/gateways/:id`             | Soft-delete + revoke MQTT credentials    |
| `GET`    | `/api/v1/devices/:id/latest`       | Latest telemetry for a device (SE/CI/SF) |
| `GET`    | `/api/v1/devices/:id/history`      | Device telemetry history (`?from=&to=`)  |
| `GET`    | `/api/v1/assignments/:id/latest`   | Latest sensor reading (ST/SP/SR/SO)      |
| `GET`    | `/api/v1/assignments/:id/history`  | Sensor history (`?from=&to=`)            |

History endpoints default to the last 24 hours when `from`/`to` are omitted.

### Build for Production

```bash
go build -o bin/server cmd/server/main.go
./bin/server
```

---

## Environment Variables

See `.env.example` for all required variables.

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=
DB_PASSWORD=
DB_NAME=

# Server
PORT=8080

# MQTT
MQTT_BROKER=localhost
MQTT_PORT=8883
MQTT_USERNAME=
MQTT_PASSWORD=
MQTT_CA_CERT=certs/ca.crt

# Only set when MQTT_BROKER differs from the certificate CN/SAN (e.g. local dev)
MQTT_TLS_SERVER_NAME=

# Set to 127.0.0.1 when Nginx runs on the same machine as Orion
TRUSTED_PROXIES=127.0.0.1

# Legacy source DB (only required for ETL commands)
SRC_DB_HOST=
SRC_DB_PORT=3306
SRC_DB_USER=
SRC_DB_PASSWORD=
SRC_DB_NAME=ima_thing

# ETL tuning (stable — put in .env)
ETL_WORKERS=1
ETL_BATCH_SLEEP_MS=200
```

---

## Registering a new Edge Device

```bash
# Via script (provisions MQTT credentials directly)
./scripts/mqtt_add_edge.sh <edge_id>

# Via API (also provisions MQTT automatically)
POST /api/v1/gateways
```

The API returns a one-time `mqtt_password` in the response — it is never stored and cannot be retrieved again.

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Commit your changes: `git commit -m 'feat: add your feature'`
4. Push: `git push origin feature/your-feature`
5. Open a Pull Request

## License

MIT

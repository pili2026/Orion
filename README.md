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
│   │   └── main.go             # Application entry point
│   └── migrate/
│       └── main.go             # Database migration runner
│
├── internal/
│   ├── config/                 # Environment variable loading
│   ├── database/               # DBManager (GORM + pgxpool)
│   ├── handler/                # HTTP handlers & MQTT subscribers
│   ├── service/                # Business logic (gateway, dynsec, mqtt)
│   ├── repository/             # Database access layer
│   ├── model/                  # GORM struct definitions
│   ├── dto/                    # Request & response structs
│   └── middleware/             # Gin middleware (auth, logging, CORS)
│
├── migrations/                 # SQL migration files (golang-migrate)
│   ├── 001_init_schema.up.sql
│   └── 001_init_schema.down.sql
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
| **Repository** | All database operations (GORM)                      |
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
# Fill in DB_USER, DB_PASSWORD, DB_NAME, MQTT_USERNAME, MQTT_PASSWORD
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

### Adding a new migration

```bash
# Create the next migration pair manually
touch migrations/002_add_your_change.up.sql
touch migrations/002_add_your_change.down.sql

# Apply
go run cmd/migrate/main.go up
```

> **Never edit an existing migration file that has already been applied.**
> Always create a new numbered migration for schema changes.

---

## API

Interactive API documentation is available via the Bruno collection in `docs/api/`.

Open Bruno → **Open Collection** → select `docs/api/`, then switch to the **local** environment.

### Available endpoints

| Method   | Path                   | Description                       |
| -------- | ---------------------- | --------------------------------- |
| `GET`    | `/health`              | Liveness probe                    |
| `POST`   | `/api/v1/gateways`     | Register a new Edge gateway       |
| `GET`    | `/api/v1/gateways`     | List all gateways                 |
| `GET`    | `/api/v1/gateways/:id` | Get a single gateway              |
| `PATCH`  | `/api/v1/gateways/:id` | Update gateway info               |
| `DELETE` | `/api/v1/gateways/:id` | Soft-delete gateway + revoke MQTT |

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
```

---

## Registering a new Edge Device

```bash
# Provision MQTT credentials and add to edge-devices group
MQTT_ADMIN_PASS=<admin_password> ./scripts/mqtt_add_edge.sh <edge_id> <edge_password>

# Or use the API (also provisions MQTT automatically)
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

# Orion

A large-scale cloud server built with Go, designed for high performance and maintainability.

## Tech Stack

| Technology                                               | Purpose                          |
| -------------------------------------------------------- | -------------------------------- |
| [Go 1.26+](https://golang.org/)                          | Primary language                 |
| [Gin](https://github.com/gin-gonic/gin)                  | HTTP web framework               |
| [GORM](https://gorm.io/)                                 | ORM for database access          |
| [pgx](https://github.com/jackc/pgx)                      | PostgreSQL driver                |
| [paho.mqtt](https://github.com/eclipse/paho.mqtt.golang) | MQTT client                      |
| [Air](https://github.com/air-verse/air)                  | Hot reload for local development |

## Project Structure

```
orion/
├── cmd/
│   └── server/
│       └── main.go         # Application entry point
│
├── internal/               # Private application code (not importable externally)
│   ├── config/             # Configuration loading (env, yaml)
│   ├── handler/            # HTTP handlers – parse requests, return responses
│   ├── service/            # Business logic layer
│   ├── repository/         # Database access layer (GORM queries)
│   ├── model/              # GORM struct definitions / DB schema
│   ├── dto/                # Request & response data transfer objects
│   └── middleware/         # Gin middleware (auth, logging, CORS, etc.)
│
├── pkg/                    # Shared utilities (safe to import from external packages)
│   ├── logger/             # Centralized logging (zap / slog)
│   └── errors/             # Custom error types
│
├── migrations/             # Database migration SQL files
│
├── deploy/
│   └── docker/             # Dockerfile and related configs
│
├── docs/
│   ├── er_diagram/         # Entity-relationship diagrams
│   └── mqtt_topics.md      # MQTT topic design and documentation
│
├── scripts/                # Development and CI/CD shell scripts
│   └── mqtt_setup.sh       # Mosquitto Dynamic Security initialization
│
├── docker-compose.yml      # Local development environment
├── go.mod                  # Go module definition and dependencies
├── go.sum                  # Dependency checksums
└── README.md
```

## Architecture

### HTTP Request Flow

```
HTTP Request → Middleware → Handler → Service → Repository → Database
```

| Layer          | Responsibility                                     |
| -------------- | -------------------------------------------------- |
| **Handler**    | Parse & validate request, return response          |
| **Service**    | Business logic, orchestrate across repositories    |
| **Repository** | All database operations                            |
| **Model**      | Database schema definitions                        |
| **DTO**        | API input/output structures (separate from models) |

### MQTT Architecture

Orion communicates with Edge devices (Talos) via MQTT over TLS (port 8883).

```
Edge (Talos)                        Orion (Cloud)
─────────────────────────────────────────────────
talos/{edge_id}/telemetry  ──────►  Subscribe talos/#
talos/{edge_id}/status     ──────►
talos/{edge_id}/event      ──────►

talos/{edge_id}/command    ◄──────  Publish talos/{id}/command
talos/{edge_id}/config     ◄──────  Publish talos/{id}/config

orion/broadcast/#          ◄──────  Publish orion/broadcast/#
```

See [docs/mqtt_topics.md](docs/mqtt_topics.md) for full topic design and payload formats.

## Getting Started

### Prerequisites

- Go 1.26+
- PostgreSQL / TimescaleDB
- MQTT Broker (Mosquitto 2.0+)
- Docker & Docker Compose

### Local Development

1. **Clone the repository**

```bash
git clone https://github.com/yourname/orion.git
cd orion
```

2. **Set up environment variables**

```bash
cp .env.example .env
# Edit .env with your local configuration
```

3. **Start dependencies**

```bash
docker-compose up -d
```

4. **Run database migrations**

```bash
go run cmd/migrate/main.go
```

5. **Start the server with hot reload**

```bash
air
```

The server will be available at `http://localhost:8080`.

### Build for Production

```bash
go build -o bin/server cmd/server/main.go
./bin/server
```

## API

Health check:

```
GET /health
```

Full API documentation can be found in `docs/`.

## Environment Variables

See `.env.example` for all required variables:

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
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -m 'feat: add your feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a Pull Request

## License

MIT

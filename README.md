# Orion

A large-scale cloud server built with Go, designed for high performance and maintainability.

## Tech Stack

| Technology                              | Purpose                          |
| --------------------------------------- | -------------------------------- |
| [Go 1.24+](https://golang.org/)         | Primary language                 |
| [Gin](https://github.com/gin-gonic/gin) | HTTP web framework               |
| [GORM](https://gorm.io/)                | ORM for database access          |
| [pgx](https://github.com/jackc/pgx)     | PostgreSQL driver                |
| [Air](https://github.com/air-verse/air) | Hot reload for local development |

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
│   └── er_diagram/         # Entity-relationship diagrams and documentation
│
├── scripts/                # Development and CI/CD shell scripts
│
├── docker-compose.yml      # Local development environment (PostgreSQL, Redis, etc.)
├── go.mod                  # Go module definition and dependencies
├── go.sum                  # Dependency checksums
└── README.md
```

## Architecture

Requests flow through the following layers:

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

## Getting Started

### Prerequisites

- Go 1.24+
- PostgreSQL
- Docker & Docker Compose (for local development)

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

3. **Start dependencies (PostgreSQL, etc.)**

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

Full API documentation can be found in `docs/` or via the Swagger UI (if enabled).

## Environment Variables

See `.env.example` for all required variables:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=
DB_PASSWORD=
DB_NAME=
PORT=8080
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -m 'feat: add your feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a Pull Request

## License

MIT

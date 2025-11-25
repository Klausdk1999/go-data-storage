# Go Data Storage API

REST API for storing and retrieving IoT sensor data with device management, signal configuration, and authentication.

## Dependencies

- **Go 1.23+**
- **PostgreSQL 16+** (or use Docker)
- **golangci-lint** (for linting - install via `make install-tools`)
- **goimports** (for formatting - install via `make install-tools`)

## Project Structure

```
go-data-storage/
├── main.go                      # Application entry point and routing
├── db.go                        # Database connection and initialization
├── models.go                    # Data models (User, Device, Signal, SignalValue)
├── auth.go                      # Authentication logic and middleware
├── auth_handler.go              # Authentication HTTP handlers (login, register device)
├── users_handler.go             # User CRUD operations
├── devices_handler.go            # Device CRUD operations
├── signals_handler.go            # Signal configuration CRUD operations
├── signal_values_handler.go      # Signal value data CRUD operations
├── user_readings_handler.go      # Legacy user readings endpoint
├── rfid_user_handler.go          # RFID user lookup endpoint
├── readings_handler.go           # Legacy readings endpoint
├── tests/                        # Test files
│   ├── auth_test.go
│   ├── handlers_test.go
│   └── models_test.go
├── migrations/                  # SQL migration files
│   ├── 001_init_schema.sql
│   ├── 002_add_devices_and_auth.sql
│   └── 003_separate_signal_values.sql
├── infra/                       # Infrastructure and server configuration
│   ├── docker-compose.full.yml   # Docker Compose for all services (DB, API, Frontend)
│   ├── docker-compose.test.yml   # Docker Compose for test database
│   └── nginx/                    # Nginx reverse proxy configuration
│       └── nginx.conf
├── scripts/                      # Utility scripts
│   └── seed.go                   # Database seeding script
├── run.sh                        # Start all services (DB, API, Frontend)
├── test.sh                       # Run tests
└── stop.sh                       # Stop all services
├── documentation/                # API documentation (Insomnia exports)
├── Makefile                      # Build and test commands
├── .golangci.yml                 # Linter configuration
└── Dockerfile                    # Docker build configuration
```

## Installation

### Prerequisites

To run this project locally, you need to install:

1. **Go 1.23+**
   - Download from: https://go.dev/dl/
   - Verify installation: `go version` (should show 1.23 or higher)
   - Set `GOPATH` and `GOROOT` if needed (usually automatic)

2. **PostgreSQL 16+** (choose one option):
   - **Option A**: Install PostgreSQL locally
     - Windows: Download from https://www.postgresql.org/download/windows/
     - macOS: `brew install postgresql@16` or download from https://www.postgresql.org/download/macosx/
     - Linux: `sudo apt-get install postgresql-16` (Ubuntu/Debian) or use your distro's package manager
   - **Option B**: Use Docker (recommended for development)
     - Install Docker Desktop: https://www.docker.com/products/docker-desktop/
     - Run: `docker-compose -f infra/docker-compose.yml up -d`

3. **Development Tools** (optional but recommended):
   - **golangci-lint**: For code linting
   - **goimports**: For code formatting
   - These will be installed via `make install-tools`

4. **Make** (optional, for using Makefile commands):
   - Windows: Install via Chocolatey (`choco install make`) or use Git Bash
   - macOS: Usually pre-installed, or `xcode-select --install`
   - Linux: Usually pre-installed, or `sudo apt-get install build-essential`

### Setup Steps

1. **Clone the repository** (if not already done):
   ```bash
   git clone <repository-url>
   cd go-data-storage
   ```

2. **Install Go dependencies**:
   ```bash
   go mod download
   go mod tidy
   ```

3. **Install development tools**:
   ```bash
   make install-tools
   # Or manually:
   # go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   # go install golang.org/x/tools/cmd/goimports@latest
   ```

4. **Set up PostgreSQL database**:
   
   If using local PostgreSQL:
   ```bash
   # Create database and user
   psql -U postgres
   CREATE DATABASE iotdb;
   CREATE USER iotuser WITH PASSWORD 'iotpassword';
   GRANT ALL PRIVILEGES ON DATABASE iotdb TO iotuser;
   \q
   ```
   
   If using Docker:
   ```bash
   docker-compose -f infra/docker-compose.yml up -d
   # Database will be created automatically
   ```

5. **Create `.env` file** in the project root:
   ```env
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=iotuser
   DB_PASSWORD=iotpassword
   DB_NAME=iotdb
   ```
   
   For Docker setup, use:
   ```env
   DB_HOST=postgres
   DB_PORT=5432
   DB_USER=iotuser
   DB_PASSWORD=iotpassword
   DB_NAME=iotdb
   ```

6. **Run database migrations** (if not using auto-migration):
   ```bash
   psql -U iotuser -d iotdb -f migrations/001_init_schema.sql
   psql -U iotuser -d iotdb -f migrations/002_add_devices_and_auth.sql
   psql -U iotuser -d iotdb -f migrations/003_separate_signal_values.sql
   ```
   
   Note: The application uses GORM auto-migration, so manual migration is usually not needed.

7. **Verify installation**:
   ```bash
   # Build the application
   make build
   # or
   go build -o bin/main ./cmd/api
   
   # Run tests
   make test
   # or
   go test ./...
   ```

8. **Run the application**:
   ```bash
   make run
   # or
   go run ./cmd/api
   ```
   
   The API will start on `http://localhost:8080` (default port).

## Configuration

Create `.env` file:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=iotuser
DB_PASSWORD=iotpassword
DB_NAME=iotdb
```

For Docker, use:
```env
DB_HOST=postgres
DB_PORT=5432
DB_USER=iotuser
DB_PASSWORD=iotpassword
DB_NAME=iotdb
```

## Commands

### Development

```bash
# Run the API
make run
# or
go run main.go

# Build the application
make build
# or
go build -o bin/main main.go
```

### Database Seeding

```bash
# Seed database with test data (user, devices, signals, signal values)
make seed
# or
go run scripts/seed.go
```

This will create:
- **1 test user**: `test@example.com` / `password123`
- **3 devices**: Temperature sensor, Humidity sensor, Smart light switch
- **5 signals**: Various analogic/digital, input/output signals
- **~97 signal values**: Historical data for testing

**Note**: The seed script will clear existing data before inserting test data.

### Testing

#### Using Bash Script (Recommended)

```bash
# Run unit tests in Docker
./test.sh

# Run integration tests (with PostgreSQL)
./test.sh --integration

# Run tests with coverage
./test.sh --coverage

# Show help
./test.sh --help
```

#### Using Makefile

```bash
# Unit tests (local Go)
make test

# Run tests with coverage
make test-coverage
# Generates: coverage.html and coverage.out

# View coverage percentage
go test -cover ./...
```

#### Using Go Directly (Local)

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
make test-coverage
# Generates: coverage.html and coverage.out
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet

# Run all quality checks
make check
```

### Docker Deployment

The easiest way to run everything is using the provided scripts:

```bash
# Start all services (DB, API, Frontend) - no seed
./run.sh

# Start all services with database seed
./run.sh --seed

# Stop all services
./stop.sh

# Run tests
./test.sh
```

Or manually using Docker Compose:

```bash
# Start all services (DB, API, Frontend)
cd infra
docker-compose -f docker-compose.full.yml up -d

# Stop all services
docker-compose -f docker-compose.full.yml down
```

# View logs
cd infra
docker-compose -f docker-compose.full.yml logs -f

# Stop services
docker-compose -f docker-compose.full.yml down
```

### Database

```bash
# Run migrations manually (if needed)
psql -U iotuser -d iotdb -f migrations/001_init_schema.sql
psql -U iotuser -d iotdb -f migrations/002_add_devices_and_auth.sql
psql -U iotuser -d iotdb -f migrations/003_separate_signal_values.sql
```

## API Endpoints

### Authentication
- `POST /auth/login` - User login (returns JWT token)
- `POST /auth/register-device` - Register new device (requires user auth)

### Users
- `GET /users` - List all users (requires auth)
- `GET /users/{id}` - Get user details (requires auth)
- `POST /users` - Create user (requires auth)
- `PUT /users/{id}` - Update user (requires auth)
- `DELETE /users/{id}` - Delete user (requires auth)

### Devices
- `GET /devices` - List all devices (requires auth)
- `GET /devices/{id}` - Get device details (requires auth)
- `POST /devices` - Create device (requires auth)
- `PUT /devices/{id}` - Update device (requires auth)
- `DELETE /devices/{id}` - Delete device (requires auth)
- `GET /devices/{device_id}/signals` - Get signals for device (requires auth)

### Signal Configurations
- `GET /signals` - List all signals (requires auth)
- `GET /signals/{id}` - Get signal details (requires auth)
- `POST /signals` - Create signal configuration (requires auth)
- `PUT /signals/{id}` - Update signal configuration (requires auth)
- `DELETE /signals/{id}` - Delete signal configuration (requires auth)

### Signal Values
- `GET /signal-values` - List signal values (requires auth)
- `GET /signal-values/{id}` - Get signal value (requires auth)
- `POST /signal-values` - Create signal value (requires user OR device auth)
- `DELETE /signal-values/{id}` - Delete signal value (requires auth)
- `GET /signals/{signal_id}/values` - Get values for signal (requires auth)

## Authentication

### User Authentication
```bash
# Login
POST /auth/login
{
  "email": "user@example.com",
  "password": "password"
}

# Response includes JWT token
# Use in subsequent requests:
Authorization: Bearer <token>
```

### Device Authentication
```bash
# Use device auth token (received when registering device)
Authorization: Bearer <device_auth_token>
```

## Example Usage

### Create User
```bash
curl -X POST http://localhost:8080/users \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com",
    "password": "securepassword"
  }'
```

### Register Device
```bash
curl -X POST http://localhost:8080/auth/register-device \
  -H "Authorization: Bearer <user_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sensor Node 1",
    "description": "Temperature sensor",
    "device_type": "sensor",
    "location": "Room 101"
  }'
```

### Create Signal Value
```bash
curl -X POST http://localhost:8080/signal-values \
  -H "Authorization: Bearer <device_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "signal_id": 1,
    "value": 23.5
  }'
```

## Testing

See `tests/` directory for test examples. Run with:
```bash
make test
```

## Docker (Standalone)

For running the API container standalone (without docker-compose):

```bash
# Build image
docker build -t iot-api .

# Run container
docker run -p 8080:8080 \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e DB_USER=iotuser \
  -e DB_PASSWORD=iotpassword \
  -e DB_NAME=iotdb \
  iot-api
```

**Note**: For production deployment, use `docker-compose` (see Docker Deployment section above) which includes PostgreSQL and proper networking.

## Troubleshooting

- **Database connection errors**: Check `.env` file and ensure PostgreSQL is running
- **Port already in use**: Change port in `cmd/main.go` or use environment variable
- **Migration errors**: Run migrations manually (see Database section)

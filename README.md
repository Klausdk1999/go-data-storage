# Go Data Storage API

REST API for storing and retrieving IoT sensor data with device management, signal configuration, and authentication.

## Dependencies

- **Go 1.25+**
- **PostgreSQL 16+** (optional - can use SQLite instead)
- **golangci-lint** (for linting - install via `make install-tools`)
- **goimports** (for formatting - install via `make install-tools`)

## Quick Start (Windows)

```powershell
.\run.ps1
```

Requirements:
- Go 1.25+ on PATH
- No C compiler required for SQLite

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
├── run.ps1                       # Start API (Windows PowerShell)
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

1. **Go 1.25+**
   - Download from: https://go.dev/dl/
   - Verify installation: `go version` (should show 1.25 or higher)
   - Set `GOPATH` and `GOROOT` if needed (usually automatic)

2. **Database** (choose one option):
   - **Option A**: SQLite (simplest, recommended for development)
     - No installation needed - SQLite is embedded
     - Database file stored in `./data/` directory
   - **Option B**: PostgreSQL 16+ (for production/scalability)
     - **Option B1**: Install PostgreSQL locally
       - Windows: Download from https://www.postgresql.org/download/windows/
       - macOS: `brew install postgresql@16` or download from https://www.postgresql.org/download/macosx/
       - Linux: `sudo apt-get install postgresql-16` (Ubuntu/Debian) or use your distro's package manager
     - **Option B2**: Use Docker (recommended for development)
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
   
   **For SQLite (simplest):**
   ```env
   DB_TYPE=sqlite
   DB_PATH=./data/storage.db
   PORT=8080
   ```
   
   **For PostgreSQL:**
   ```env
   DB_TYPE=postgres
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=iotuser
   DB_PASSWORD=iotpassword
   DB_NAME=iotdb
   PORT=8080
   ```
   
   **With TTN MQTT integration (optional):**
   ```env
   DB_TYPE=sqlite
   DB_PATH=./data/storage.db
   PORT=8080
   
   # TTN MQTT Configuration
   TTN_MQTT_BROKER=mqtt://nam1.cloud.thethings.network:1883
   TTN_USERNAME=your-ttn-application-id@ttn
   TTN_PASSWORD=your-ttn-api-key
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
   
   **Important for SQLite:** SQLite requires CGO to be enabled. Set `CGO_ENABLED=1` before running:
   
   **Windows PowerShell:**
   ```powershell
   $env:CGO_ENABLED = "1"
   go run ./cmd/api
   ```
   
   **Windows CMD:**
   ```cmd
   set CGO_ENABLED=1
   go run ./cmd/api
   ```
   
   **Linux/macOS:**
   ```bash
   CGO_ENABLED=1 go run ./cmd/api
   ```
   
   **Windows WSL (recommended for SQLite on Windows):**
   ```bash
   # From PowerShell/CMD, run:
   wsl bash run-wsl.sh
   
   # Or manually:
   wsl bash -c "export PATH=/usr/local/go/bin:\$PATH && export CGO_ENABLED=1 && cd '/mnt/c/Users/Klaus/Documents/Mestrado CA/go-data-storage' && go run ./cmd/api"
   ```
   
   **Note:** WSL already has `gcc` installed, so CGO works out of the box. This is the easiest way to run with SQLite on Windows without installing a C compiler.
   
   **Note:** If you get CGO errors, you may need a C compiler:
   - **Windows**: Install TDM-GCC, MinGW, or use `choco install mingw`
   - **macOS**: Install Xcode Command Line Tools: `xcode-select --install`
   - **Linux**: Install `gcc` via your package manager
   
   The API will start on `http://localhost:8080` (default port, configurable via `PORT` env var).
   
   **Alternative:** Use PostgreSQL instead of SQLite (doesn't require CGO):
   ```env
   DB_TYPE=postgres
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=iotuser
   DB_PASSWORD=iotpassword
   DB_NAME=iotdb
   ```

## Configuration

Create `.env` file in the project root. See setup steps above for examples.

**Environment Variables:**
- `DB_TYPE` - Database type: `sqlite` or `postgres` (default: `postgres`)
- `DB_PATH` - SQLite database file path (required if `DB_TYPE=sqlite`)
- `DB_HOST` - PostgreSQL host (required if `DB_TYPE=postgres`)
- `DB_PORT` - PostgreSQL port (default: `5432`)
- `DB_USER` - PostgreSQL username (required if `DB_TYPE=postgres`)
- `DB_PASSWORD` - PostgreSQL password (required if `DB_TYPE=postgres`)
- `DB_NAME` - PostgreSQL database name (required if `DB_TYPE=postgres`)
- `PORT` - API server port (default: `8080`)
- `TTN_MQTT_BROKER` - TTN MQTT broker URL (optional, for TTN integration)
- `TTN_USERNAME` - TTN application username (optional)
- `TTN_PASSWORD` - TTN API key (optional)

## Commands

### Development

**For SQLite (requires CGO_ENABLED=1):**

**Windows PowerShell:**
```powershell
$env:CGO_ENABLED = "1"
go run ./cmd/api
```

**Windows CMD:**
```cmd
set CGO_ENABLED=1
go run ./cmd/api
```

**Linux/macOS:**
```bash
CGO_ENABLED=1 go run ./cmd/api
```

**Windows WSL (recommended for SQLite):**
```bash
# Use the provided script:
wsl bash run-wsl.sh

# Or manually:
wsl bash -c "export PATH=/usr/local/go/bin:\$PATH && export CGO_ENABLED=1 && cd '/mnt/c/Users/Klaus/Documents/Mestrado CA/go-data-storage' && go run ./cmd/api"
```

**Build:**
```bash
# Windows PowerShell
$env:CGO_ENABLED = "1"
go build -o bin/api.exe ./cmd/api

# Linux/macOS
CGO_ENABLED=1 go build -o bin/api ./cmd/api
```

**For PostgreSQL (doesn't require CGO):**
```bash
go run ./cmd/api
```

**Note:** The `make run` command requires a Makefile. If you don't have `make` installed, use the commands above directly.

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

### TTN River Monitoring Endpoints
- `GET /ttn/uplinks` - List TTN uplinks with filters (device_id, start_date, end_date, limit)
- `GET /ttn/devices` - List all TTN devices with statistics
- `GET /ttn/stats` - Get TTN statistics (total uplinks, unique devices, date range)

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

## TTN MQTT Integration

The API supports automatic data collection from The Things Network (TTN) via MQTT.

**Features:**
- Automatically subscribes to TTN uplink messages
- Decodes binary sensor payloads (TF02-Pro LiDAR format)
- Stores data in the database as SignalValue records
- Auto-creates Device and Signal entries for TTN devices

**Configuration:**
Add to `.env`:
```env
TTN_MQTT_BROKER=mqtt://nam1.cloud.thethings.network:1883
TTN_USERNAME=your-application-id@ttn
TTN_PASSWORD=your-ttn-api-key
```

The MQTT client will automatically connect on startup if credentials are provided.

## Troubleshooting

- **Database connection errors**: 
  - Check `.env` file and database credentials
  - For SQLite: Ensure the `data/` directory exists and is writable
  - For PostgreSQL: Ensure PostgreSQL is running and accessible
- **Port already in use**: Change `PORT` environment variable or kill the process using port 8080
- **MQTT connection fails**: Check TTN credentials and broker URL. API will continue without MQTT if connection fails
- **Migration errors**: GORM auto-migrates on startup. For manual migrations, see Database section

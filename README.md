# Go Data Storage API

REST API for storing and retrieving IoT sensor data with device management, signal configuration, and authentication.

## Dependencies

- **Go 1.21+**
- **PostgreSQL 16+** (or use Docker)
- **golangci-lint** (for linting - install via `make install-tools`)

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
│   ├── docker-compose.yml        # Docker Compose for API and PostgreSQL
│   ├── docker-compose.infrastructure.yml  # Infrastructure services (Nginx, Portainer, Nextcloud)
│   └── nginx/                    # Nginx reverse proxy configuration
│       └── nginx.conf
├── scripts/                      # Utility scripts
│   ├── start.ps1                 # Start main services
│   ├── start-infrastructure.ps1  # Start infrastructure services
│   └── setup-remote-access.ps1   # Setup remote access
├── documentation/                # API documentation (Insomnia exports)
├── Makefile                      # Build and test commands
├── .golangci.yml                 # Linter configuration
└── Dockerfile                    # Docker build configuration
```

## Installation

```bash
# Install Go dependencies
go mod download

# Install development tools
make install-tools
```

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

### Testing

```bash
# Run all tests
make test
# or
go test ./...

# Run tests with coverage
make test-coverage
# Generates: coverage.html and coverage.out

# View coverage percentage
go test -cover ./...
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

```bash
# Start API and PostgreSQL using Docker Compose
.\scripts\start.ps1
# or
docker-compose -f infra/docker-compose.yml up -d

# Start infrastructure services (Nginx, Portainer, Nextcloud)
.\scripts\start-infrastructure.ps1
# or
docker-compose -f infra/docker-compose.infrastructure.yml up -d

# View logs
docker-compose -f infra/docker-compose.yml logs -f

# Stop services
docker-compose -f infra/docker-compose.yml down
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

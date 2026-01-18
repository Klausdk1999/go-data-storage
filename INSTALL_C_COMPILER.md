# Installing C Compiler for SQLite Support

SQLite support in Go requires CGO, which needs a C compiler. This guide helps you install one.

## Option 1: TDM-GCC (Recommended - Easiest)

1. **Download TDM-GCC:**
   - Visit: https://jmeubank.github.io/tdm-gcc/
   - Download the latest 64-bit installer

2. **Install:**
   - Run the installer
   - Choose "Full installation"
   - Default installation path: `C:\TDM-GCC-64`
   - **Important**: Make sure "Add to PATH" is checked during installation

3. **Restart your terminal/PowerShell**

4. **Verify installation:**
   ```powershell
   gcc --version
   ```

5. **Run the API:**
   ```powershell
   $env:CGO_ENABLED = "1"
   go run ./cmd/api
   ```

## Option 2: MinGW via Chocolatey

If you have Chocolatey installed:

```powershell
choco install mingw -y
```

Then restart your terminal and run:
```powershell
$env:CGO_ENABLED = "1"
go run ./cmd/api
```

## Option 3: Use PostgreSQL Instead (No C Compiler Needed)

If you don't want to install a C compiler, switch to PostgreSQL:

1. **Install PostgreSQL** (if not already installed)
   - Download from: https://www.postgresql.org/download/windows/
   - Or use Docker: `docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=iotpassword postgres:16`

2. **Update `.env` file:**
   ```env
   DB_TYPE=postgres
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=iotpassword
   DB_NAME=iotdb
   ```

3. **Create database:**
   ```sql
   CREATE DATABASE iotdb;
   ```

4. **Run API (no CGO needed):**
   ```powershell
   go run ./cmd/api
   ```

## Option 4: Use Docker with PostgreSQL

If you have Docker installed:

```bash
# Start PostgreSQL
docker run -d --name postgres-iot \
  -p 5432:5432 \
  -e POSTGRES_USER=iotuser \
  -e POSTGRES_PASSWORD=iotpassword \
  -e POSTGRES_DB=iotdb \
  postgres:16

# Update .env to use PostgreSQL (see Option 3)

# Run API
go run ./cmd/api
```

## Quick Check

After installing a C compiler, verify it works:

```powershell
gcc --version
```

If you see version output, you're good to go!

## Current Status

Your `.env` is currently configured for SQLite:
```
DB_TYPE=sqlite
DB_PATH=./data/ttn-data.db
```

To use SQLite, you **must** have a C compiler installed.

## Recommendation

- **For development**: Use TDM-GCC (Option 1) - it's the easiest and works well
- **For production**: Consider PostgreSQL for better performance and features
- **For quick testing**: Use Docker PostgreSQL (Option 4)

[![Go](https://github.com/dydanz/recon-engine/actions/workflows/go.yml/badge.svg)](https://github.com/dydanz/recon-engine/actions/workflows/go.yml)

# Transaction Reconciliation Engine

A transaction reconciliation service built with Go/Gin that identifies unmatched and discrepant transactions between internal system data and external bank statements.

## Architecture

The service follows a **Layered Architecture** pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                     API Layer (Gin)                         │
│                  Handlers + Middleware                      │
└─────────────────────────────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                   Business Logic Layer                      │
│              Services (Transaction, Reconciliation)         │
└─────────────────────────────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                   Processing Layer                          │
│         Matcher Engine + CSV Parser + Validators            │
└─────────────────────────────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                   Data Access Layer                         │
│           Repositories (Transaction, Reconciliation)        │
└─────────────────────────────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                     PostgreSQL Database                     │
└─────────────────────────────────────────────────────────────┘
```

### Design Patterns Used

1. **Strategy Pattern**: Pluggable matching strategies (exact match, fuzzy match, etc.)
2. **Repository Pattern**: Abstraction over data access logic
3. **Builder Pattern**: Constructing reconciliation reports with flexible output
4. **Pipeline Pattern**: Data flows through Parser → Validator → Matcher → Reporter

## Core Algorithm

The reconciliation engine uses a **Two-Phase Hash-Based Matching** approach:

### Phase 1: Load & Index
```
System Transactions → HashMap<TrxID, Transaction>
Bank Statements     → HashMap<TrxRefID, Statement>
```

### Phase 2: Match & Compare
```
For each system transaction:
  1. Lookup in bank map (O(1))
  2. Compare amounts and dates
  3. Categorise: matched | unmatched | discrepancy
```

**Time Complexity**: O(n + m) where n = system transactions, m = bank statements
**Space Complexity**: O(n + m) for hash maps

## Database Schema

### Transactions Table
```sql
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    trx_id VARCHAR(255) UNIQUE NOT NULL,
    amount DECIMAL(20, 2) NOT NULL,
    type VARCHAR(10) NOT NULL,  -- DEBIT or CREDIT
    transaction_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Reconciliation Jobs Table
```sql
CREATE TABLE reconciliation_jobs (
    id SERIAL PRIMARY KEY,
    job_id UUID UNIQUE NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL,  -- PENDING, PROCESSING, COMPLETED, FAILED
    total_processed INT DEFAULT 0,
    total_matched INT DEFAULT 0,
    total_unmatched INT DEFAULT 0,
    total_discrepancies DECIMAL(20, 2) DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Reconciliation Results Table
```sql
CREATE TABLE reconciliation_results (
    id SERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES reconciliation_jobs(job_id),
    trx_id VARCHAR(255),
    trx_ref_id VARCHAR(255),
    system_amount DECIMAL(20, 2),
    bank_amount DECIMAL(20, 2),
    discrepancy DECIMAL(20, 2),
    match_status VARCHAR(20) NOT NULL,  -- MATCHED, UNMATCHED_SYSTEM, UNMATCHED_BANK, DISCREPANCY
    bank_source VARCHAR(255),
    transaction_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Indexes**: Optimized for fast lookups on `trx_id`, `transaction_time`, `job_id`, and `match_status`

## Setup Instructions

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Docker & Docker Compose (optional)
- Python 3.8+ (for mock data generation)

### Option 1: Using Docker Compose (Recommended)

1. **Clone the repository**
```bash
cd recon-engine
```

2. **Copy environment file**
```bash
cp .env.example .env
```

3. **Start services**
```bash
docker-compose up -d
```

This will start:
- PostgreSQL database on port 5434 (mapped from container port 5432)
- API service on port 8080

**Note**: If port 5432 is already in use on your system, the compose file uses port 5434 instead.

4. **Verify services are running**
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"healthy"}
```

### Option 2: Local Development Setup

1. **Install dependencies**
```bash
go mod download
```

2. **Setup PostgreSQL database**
```bash
# Create database
createdb recon_db

# Run migrations
psql -d recon_db -f migrations/001_init_schema.sql
```

3. **Configure environment variables**
```bash
cp .env.example .env
# Edit .env if needed
```

4. **Generate Swagger docs**
```bash
# Install swag if not already installed
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs (required before building)
make swag
```

5. **Build and run**
```bash
make build
./bin/recon-engine
```

Or run directly:
```bash
make run
```

## Generating Mock Data

Generate test data - (x) system transactions + each of the bank's CSV files with (y) rows each (configurable):

```bash
# Ensure Docker containers are running
docker-compose up -d

# Generate mock data
python3 scripts/generate_mock_data.py
```

This creates:
- **PostgreSQL Database**: 10,000 transactions in `recon_db.transactions` table
- `test/testdata/bank_*.csv` - bank statement CSV files 

**Note**: The script reads database configuration from `.env` file and connects to PostgreSQL on port 5434 (as configured in docker-compose.yml).

## API Documentation

### Swagger UI

Access interactive API documentation at:
```
http://localhost:8080/swagger/index.html
```

### Generate Swagger Docs

```bash
# Install swag CLI (if not already installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs (required before building)
swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

# Or use make
make swag
```

**Note**: You must run `swag init` before building the application, as the API depends on the generated docs package.

### API Endpoints

#### 1. Create Transaction
```http
POST /api/v1/transactions
Content-Type: application/json

{
  "trx_id": "TRX00001",
  "amount": 1000.50,
  "type": "DEBIT",
  "transaction_time": "2024-01-15T10:30:00Z"
}
```

#### 2. Bulk Create Transactions
```http
POST /api/v1/transactions/bulk
Content-Type: application/json

{
  "transactions": [
    {
      "trx_id": "TRX00001",
      "amount": 1000.50,
      "type": "DEBIT",
      "transaction_time": "2024-01-15T10:30:00Z"
    },
    ...
  ]
}
```

#### 3. Get Transaction by ID
```http
GET /api/v1/transactions/{trx_id}
```

#### 4. Get Transactions by Date Range
```http
GET /api/v1/transactions?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z
```

#### 5. Perform Reconciliation
```http
POST /api/v1/reconcile
Content-Type: application/json

{
  "system_file_path": "",
  "bank_file_paths": [
    "/app/testdata/bank_bca.csv",
    "/app/testdata/bank_mandiri.csv"
  ],
  "start_date": "2024-01-01",
  "end_date": "2024-12-31"
}

# Note: When using Docker, CSV file paths should start with /app/testdata/
# When running locally, use test/testdata/
```

**Response:**
```json
{
  "success": true,
  "message": "Reconciliation completed successfully",
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "total_processed": 12000,
    "total_matched": 10500,
    "total_unmatched": 1500,
    "total_discrepancies": 15000.50,
    "unmatched_system": [...],
    "unmatched_bank": {
      "bank_bca.csv": [...],
      "bank_mandiri.csv": [...]
    },
    "discrepancies": [...]
  }
}
```

#### 6. Get Job Status
```http
GET /api/v1/reconcile/jobs/{job_id}
```

#### 7. Get Job Summary
```http
GET /api/v1/reconcile/jobs/{job_id}/summary
```

### Response Format

All API responses follow a standardized format:

**Success Response:**
```json
{
  "success": true,
  "message": "Operation completed successfully",
  "data": { ... }
}
```

**Error Response:**
```json
{
  "success": false,
  "message": "Operation failed",
  "error": {
    "code": "ERROR_CODE",
    "message": "Detailed error message",
    "details": "Additional error context"
  }
}
```

## CSV File Formats

### System Transactions CSV
```csv
trx_id,amount,type,transaction_time
TRX00001,1000.50,DEBIT,2024-01-15T10:30:00Z
TRX00002,2500.00,CREDIT,2024-01-16T14:20:00Z
```

**Required Columns:**
- `trx_id`: Unique transaction identifier
- `amount`: Positive decimal number
- `type`: Either "DEBIT" or "CREDIT"
- `transaction_time`: ISO 8601 datetime format

### Bank Statement CSV
```csv
trx_ref_id,amount,date
TRX00001,-1000.50,2024-01-15
TRX00002,2500.00,2024-01-16
```

**Required Columns:**
- `trx_ref_id`: Transaction reference (matches system trx_id)
- `amount`: Can be negative (debits) or positive (credits)
- `date`: Date in YYYY-MM-DD format

**Supported Date Formats:**
- `2024-01-15`
- `2024-01-15 10:30:00`
- `15/01/2024`
- `01/15/2024`
- ISO 8601 (RFC3339)

## Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -v ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out
```

## Performance Optimisation

### For Large Files (Millions of Records)

1. **Adjust Batch Size**: Increase `BATCH_SIZE` environment variable
```bash
export BATCH_SIZE=50000
```

2. **Database Tuning**: Optimize PostgreSQL settings
```sql
-- Increase work memory
SET work_mem = '256MB';

-- Increase shared buffers
SET shared_buffers = '1GB';
```

3. **Parallel Processing**: The engine processes transactions in batches, allowing for parallel processing of multiple reconciliation jobs

### Memory Management

The CSV parser uses **streaming** to avoid loading entire files into memory:
- Processes files in configurable batches (default: 10,000 rows)
- Each batch is processed and cleared before loading the next
- Suitable for files with millions of records

## Error Handling

The service implements robust error handling:

1. **Invalid CSV Rows**: Logs warning and continues processing
2. **Data Corruption**: Skips corrupted rows and tracks errors
3. **Database Errors**: Returns appropriate HTTP status codes
4. **Validation Errors**: Returns detailed validation messages

Example error log:
```json
{
  "level": "warning",
  "msg": "Failed to parse record, skipping",
  "line": 1523,
  "error": "invalid amount: can't convert invalid to decimal",
  "time": "2024-01-15T10:30:00Z"
}
```

## Logging

Structured JSON logging with log levels:
- `debug`: Detailed debugging information
- `info`: General informational messages
- `warn`: Warning messages (non-critical errors)
- `error`: Error messages (critical errors)

Set log level via environment:
```bash
export LOG_LEVEL=debug
```

## Project Structure

```
recon-engine/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── config/                     # Configuration management
│   ├── domain/                     # Domain models and entities
│   ├── handler/                    # HTTP handlers (controllers)
│   ├── middleware/                 # HTTP middleware
│   ├── matcher/                    # Reconciliation engine
│   ├── parser/                     # CSV parsers
│   ├── repository/                 # Data access layer
│   └── service/                    # Business logic layer
├── pkg/
│   ├── logger/                     # Logging utilities
│   ├── response/                   # HTTP response helpers
│   └── validator/                  # Validation utilities
├── migrations/                     # Database migrations
├── scripts/                        # Utility scripts
│   └── generate_mock_data.py      # Mock data generator
├── test/                          # Tests and test data
│   ├── testdata/                  # Generated test files
│   ├── matcher_test.go            # Matcher tests
│   └── parser_test.go             # Parser tests
├── docker-compose.yml             # Docker composition
├── Dockerfile                     # Container image
├── Makefile                       # Build automation
├── go.mod                         # Go dependencies
└── README.md                      # This file
```

## Rollback Functionality

Database transactions are used for data integrity:

1. **Bulk Insert Transactions**: Uses database transactions with rollback on error
2. **Reconciliation Jobs**: Can be marked as FAILED and retried
3. **Migration Rollback**: Use `make migrate-down` to rollback schema

## Troubleshooting

### Database Connection Issues
```bash
# Check if PostgreSQL is running
docker-compose ps

# View logs
docker-compose logs postgres

# Restart database
docker-compose restart postgres
```

### Port Already in Use
```bash
# Change port in .env
SERVER_PORT=8081

# Or use different port in docker-compose.yml
```

### Permission Issues with CSV Files
```bash
# Ensure files are readable
chmod 644 test/testdata/*.csv
```

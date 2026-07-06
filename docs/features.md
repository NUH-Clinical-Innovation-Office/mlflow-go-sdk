# Features

## Core Features

| Feature | Status | Description |
|---------|--------|-------------|
| Chi Router | stable | Lightweight, idiomatic HTTP routing |
| sqlc | stable | Type-safe SQL code generation from SQL queries |
| PostgreSQL | stable | Primary database with pgx/v5 driver |
| JWT Authentication | stable | Secure token-based auth with bcrypt password hashing |
| Approved Users Gate | stable | Email whitelist for controlled user registration |
| OpenTelemetry | stable | Distributed tracing with OTLP support |
| Zap Logging | stable | Structured, high-performance logging |
| Database Migrations | stable | Using golang-migrate for schema management |
| Docker Support | stable | Multi-stage builds and docker-compose |
| CORS Middleware | stable | Cross-origin request handling with configurable origins |
| Request ID Middleware | stable | Unique request ID per request for tracing |
| Real IP Middleware | stable | Extracts real client IP from proxy headers |
| Timeout Middleware | stable | 30-second request timeout |
| Integration Tests | stable | testcontainers-go for real database testing |

## API Features

| Feature | Status | Description |
|---------|--------|-------------|
| User Registration | stable | POST /api/v1/auth/register (requires approved_id) |
| User Login | stable | POST /api/v1/auth/login |
| Get Current User | stable | GET /api/v1/me |
| Todo CRUD | stable | Full todo management with user ownership (supports title, description, is_completed, due_date) |
| Approved Users Admin | stable | Admin management of registration whitelist (single + bulk create) |

## Planned Features

| Feature | Status | Description |
|---------|--------|-------------|
| Refresh Tokens | planned | JWT refresh token flow |
| WebSocket Support | planned | Real-time communication |
| Email Verification | planned | Email-based user verification |
| Password Reset | planned | Forgotten password flow |
| Rate Limiting | planned | Configurable rate limiting (config struct exists, not yet wired) |

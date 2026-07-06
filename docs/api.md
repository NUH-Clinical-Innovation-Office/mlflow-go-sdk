# API Reference

## Base URL

```
http://localhost:8080
```

## Authentication

Most endpoints require JWT Bearer token authentication:

```
Authorization: Bearer <token>
```

Admin endpoints require the `admin` role in the JWT claims.

## Public Endpoints

### Health Check

```
GET /health
```

Returns database connectivity status.

**Response (200 - Healthy):**
```json
{
  "status": "healthy"
}
```

**Response (503 - Unhealthy):**
```json
{
  "status": "unhealthy"
}
```

### Root

```
GET /
```

Returns API version and status information.

**Response (200):**
```json
{
  "version": "1.0.0",
  "status": "running"
}
```

---

## Authentication

### Register

```
POST /api/v1/auth/register
```

Register a new user. Only users with pre-approved emails can register. Requires an `approved_id` obtained from an admin.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123",
  "approved_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Password Requirements:**
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit

**Response (201):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "bearer"
}
```

**Errors:**
- `400` - Invalid request body, invalid password, or invalid approved_id format
- `404` - Approved user not found
- `409` - Email already registered

---

### Login

```
POST /api/v1/auth/login
```

Authenticate and receive a JWT token.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123"
}
```

**Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "bearer"
}
```

**Errors:**
- `400` - Invalid request body
- `401` - Invalid credentials

---

## User (Authenticated)

### Get Current User

```
GET /api/v1/me
```

Get the currently authenticated user's profile.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "first_name": "John",
  "is_active": true,
  "roles": ["user"],
  "created_at": "2024-01-01T00:00:00Z"
}
```

**Errors:**
- `401` - Unauthorized (missing or invalid token)

---

## Todos

All todo endpoints require authentication. Todos are user-scoped - users can only access their own todos.

### List Todos

```
GET /api/v1/todos
```

List all todos for the authenticated user.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (200):**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "550e8400-e29b-41d4-a716-446655440001",
    "title": "Todo title",
    "description": "Optional description",
    "is_completed": false,
    "due_date": "2024-12-31T23:59:59Z",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

**Fields:**
- `due_date` and `description` are nullable (may be `null`)

---

### Create Todo

```
POST /api/v1/todos
```

Create a new todo for the authenticated user.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "title": "New todo",
  "description": "Optional description",
  "due_date": "2024-12-31T23:59:59Z"
}
```

**Validation:**
- `title` is required (max 500 characters)
- `description` is optional
- `due_date` is optional (RFC3339 format)

**Response (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "550e8400-e29b-41d4-a716-446655440001",
  "title": "New todo",
  "description": "Optional description",
  "is_completed": false,
  "due_date": "2024-12-31T23:59:59Z",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

---

### Get Todo by ID

```
GET /api/v1/todos/{id}
```

Get a single todo by ID. Returns 404 if the todo doesn't exist or belongs to another user.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "550e8400-e29b-41d4-a716-446655440001",
  "title": "Todo title",
  "description": "Optional description",
  "is_completed": false,
  "due_date": "2024-12-31T23:59:59Z",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**Errors:**
- `400` - Invalid UUID format
- `404` - Todo not found or not owned by user

---

### Update Todo

```
PUT /api/v1/todos/{id}
```

Update an existing todo.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "title": "Updated title",
  "description": "Updated description",
  "is_completed": true,
  "due_date": "2024-12-31T23:59:59Z"
}
```

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "550e8400-e29b-41d4-a716-446655440001",
  "title": "Updated title",
  "description": "Updated description",
  "is_completed": true,
  "due_date": "2024-12-31T23:59:59Z",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-02T00:00:00Z"
}
```

**Errors:**
- `400` - Invalid UUID format or invalid request body
- `404` - Todo not found or not owned by user

---

### Delete Todo

```
DELETE /api/v1/todos/{id}
```

Delete a todo.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (204):** No content

**Errors:**
- `400` - Invalid UUID format
- `404` - Todo not found or not owned by user

---

## Admin (Admin Role Required)

All admin endpoints require `Authorization: Bearer <token>` with the `admin` role.

### List Approved Users

```
GET /api/v1/admin/approved-users
```

List all users in the registration whitelist.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (200):**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "approved@example.com",
    "first_name": "Jane",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Create Approved User

```
POST /api/v1/admin/approved-users
```

Add a user to the registration whitelist. New users need the returned `id` as their `approved_id` during registration.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "email": "newuser@example.com",
  "first_name": "New"
}
```

**Validation:**
- `email` must be valid email format
- `first_name` is required (letters, spaces, hyphens, apostrophes only)

**Response (201):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "newuser@example.com",
  "first_name": "New",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**Errors:**
- `400` - Invalid request body
- `409` - Email already in approved list

---

### Bulk Create Approved Users

```
POST /api/v1/admin/approved-users/bulk
```

Add multiple users to the registration whitelist in a single request.

**Headers:**
```
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "users": [
    {
      "email": "user1@example.com",
      "first_name": "Alice"
    },
    {
      "email": "user2@example.com",
      "first_name": "Bob"
    }
  ]
}
```

**Response (201):**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user1@example.com",
    "first_name": "Alice",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  },
  {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "email": "user2@example.com",
    "first_name": "Bob",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

**Errors:**
- `400` - Invalid request body or empty users array

---

### Delete Approved User

```
DELETE /api/v1/admin/approved-users/{id}
```

Remove a user from the registration whitelist.

**Headers:**
```
Authorization: Bearer <token>
```

**Response (204):** No content

**Errors:**
- `400` - Invalid UUID format

---

## Error Response Format

All errors return a JSON body:

```json
{
  "error": "Error message here"
}
```

Common HTTP status codes:
- `400` - Bad Request (validation errors, invalid format)
- `401` - Unauthorized (missing or invalid credentials)
- `403` - Forbidden (insufficient permissions, e.g. admin role required)
- `404` - Not Found
- `409` - Conflict (resource already exists)
- `500` - Internal Server Error

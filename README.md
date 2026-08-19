# Backend Golang Coding Test

This repository contains the implementation for the 7 Solutions backend coding test.

## Contents

- User Management REST API written in Go
- MongoDB persistence using the official MongoDB Go driver
- JWT authentication with HS256
- Request logging middleware
- Background user-count logger running every 10 seconds
- Unit tests using Go's standard `testing` package and mocked repositories
- Docker and Docker Compose setup
- Lottery search design proposal in [docs/lottery-search-design.md](docs/lottery-search-design.md)

## Architecture

The API uses a small layered structure:

- `internal/domain`: user entity and domain errors
- `internal/ports`: repository interface used by the application layer
- `internal/application`: user use cases and validation
- `internal/adapters/mongo`: MongoDB repository implementation
- `internal/auth`: password hashing and JWT management
- `internal/httpapi`: HTTP handlers and middleware
- `internal/background`: scheduled user-count logger

The repository interface keeps business logic decoupled from MongoDB and makes unit tests fast.

## Requirements

- Go 1.25+
- MongoDB 7+ if running locally without Docker
- Docker and Docker Compose for containerized execution

## Configuration

Environment variables:

| Name | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | API port |
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection URI |
| `MONGO_DATABASE` | `backend_challenge` | Mongo database name |
| `JWT_SECRET` | required | HS256 signing secret |
| `JWT_TTL_HOURS` | `24` | JWT lifetime in hours |

## Run Locally

Start MongoDB, then run:

```sh
go mod tidy
JWT_SECRET=local-dev-secret go run .
```

Health check:

```sh
curl http://localhost:8080/healthz
```

## Run With Docker Compose

```sh
docker compose up --build
```

The API will be available at `http://localhost:8080`.

## Tests

```sh
go test ./...
```

The tests mock repository behavior and do not require MongoDB.

## JWT Guide

Register a user:

```sh
curl -s -X POST http://localhost:8080/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"password123"}'
```

Login to receive a JWT:

```sh
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"password123"}' | jq -r .token)
```

Use the token on protected endpoints:

```sh
curl -s http://localhost:8080/users \
  -H "Authorization: Bearer $TOKEN"
```

Tokens are signed with HMAC SHA-256 (`HS256`) using `JWT_SECRET`.

## API

### Public

`GET /healthz`

Response:

```json
{"status":"ok"}
```

`POST /register`

Request:

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "password": "password123"
}
```

Response `201`:

```json
{
  "id": "66b3f6c5d70a8f77f1b6e010",
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "created_at": "2026-08-18T14:00:00Z"
}
```

`POST /login`

Request:

```json
{
  "email": "ada@example.com",
  "password": "password123"
}
```

Response `200`:

```json
{
  "token": "<jwt>",
  "user": {
    "id": "66b3f6c5d70a8f77f1b6e010",
    "name": "Ada Lovelace",
    "email": "ada@example.com",
    "created_at": "2026-08-18T14:00:00Z"
  }
}
```

### Protected

All protected routes require:

```text
Authorization: Bearer <jwt>
```

`POST /users`

Creates a user with the same request body as `/register`.

`GET /users`

Lists all users.

`GET /users/{id}`

Fetches a user by ID.

`PATCH /users/{id}`

Request:

```json
{
  "name": "Augusta Ada King",
  "email": "ada.king@example.com"
}
```

Both fields are optional, but at least one field must be present.

`DELETE /users/{id}`

Deletes a user and returns `204 No Content`.

## Validation And Assumptions

- Email addresses are normalized to lowercase.
- Email must be unique and is enforced by a MongoDB unique index.
- Passwords must be at least 8 characters and are hashed with bcrypt.
- The API has authentication but no role model. Any authenticated user can call the protected user-management endpoints.
- IDs are generated as MongoDB ObjectID hex strings in the application layer.
- The background user-count logger starts after MongoDB connectivity and index creation succeed.

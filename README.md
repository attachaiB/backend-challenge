# User Management API

A REST API in Go for managing users, backed by MongoDB, with JWT authentication.

- Part 1 (code): this project.
- Part 2 (design only): [docs/LOTTERY_DESIGN.md](docs/LOTTERY_DESIGN.md).
- The original challenge statement: [CHALLENGE.md](CHALLENGE.md).

## Tech stack

| Concern | Choice |
| --- | --- |
| Language | Go 1.24 |
| HTTP framework | Gin |
| Database | MongoDB 7, official driver `go.mongodb.org/mongo-driver/v2` |
| Auth | JWT HS256 (`github.com/golang-jwt/jwt/v5`), bcrypt password hashing |
| Validation | Gin binding tags (`go-playground/validator`) |
| Tests | Standard `testing` package with hand-written fakes |

## Running

### With Docker (API + MongoDB)

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`. The compose file uses a development
JWT secret; set `JWT_SECRET` in your shell to override it.

### Locally

Start MongoDB, then:

```bash
cp .env.example .env        # edit JWT_SECRET
export $(grep -v '^#' .env | xargs)
go run .
```

### Configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `PORT` | `8080` | |
| `MONGO_URI` | `mongodb://localhost:27017` | |
| `MONGO_DATABASE` | `user_management` | |
| `JWT_SECRET` | none | **Required**, at least 32 characters. The API refuses to start without it. |
| `JWT_DURATION` | `24h` | Token lifetime, Go duration format |
| `USER_COUNT_INTERVAL` | `10s` | How often the background job logs the user count |
| `SHUTDOWN_TIMEOUT` | `10s` | How long to wait for running requests on shutdown |

### Tests

```bash
go test ./... -race
```

The tests need no database. MongoDB is replaced by an in-memory fake of the
`UserRepository` interface, and the HTTP tests use fakes of the service interfaces.

## Project structure

Requests flow through the layers top to bottom. Each layer depends only on the
interface of the layer below it, so each one can be tested on its own.

```
main.go                     wiring (dependency injection) and graceful shutdown
internal/
  routes/                   URL table, route groups, which routes need a JWT
  middleware/               auth.go (JWT check), logger.go (method, path, status, duration)
  controller/               binds JSON into models, calls the service, maps errors to HTTP status
  service/                  business rules: hashing, normalizing input, issuing and parsing JWTs
    interface.go            AuthService, UserService
  repository/               the only code that talks to MongoDB
    interface.go            UserRepository
  model/                    request/response shapes (json tags) and error values
  entity/                   MongoDB documents (bson tags)
  database/                 connection and the unique email index
  worker/                   background job that logs the user count
  config/                   environment variables
```

`model` and `entity` are kept separate: the controller only sees models, so the
password hash and the document layout never reach the client.

## Using JWT

1. Register a user (public endpoint):

   ```bash
   curl -X POST localhost:8080/api/auth/register \
     -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
   ```

2. Log in to get a token:

   ```bash
   curl -X POST localhost:8080/api/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"email":"alice@example.com","password":"password123"}'
   ```

   ```json
   {
     "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
     "token_type": "Bearer",
     "expires_at": "2026-11-13T04:01:04Z"
   }
   ```

3. Send the token in the `Authorization` header on every `/api/users` request:

   ```bash
   TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
   curl localhost:8080/api/users -H "Authorization: Bearer $TOKEN"
   ```

How the token works:

- Signed with HMAC-SHA256 (`HS256`) using `JWT_SECRET`.
- Claims: `sub` (user ID), `iat` (issued at), `exp` (expiry).
- The middleware accepts **only** `HS256`, so a token that switches the
  algorithm (for example to `none`) is rejected. Tokens without `exp`, expired
  tokens and tokens signed with another secret are rejected too.
- A rejected or missing token returns `401`.

## API

All responses are JSON. Errors always have the shape
`{"error": "...", "field": "..."}`, where `field` is present when one input is at fault.

| Method | Path | Auth | Success |
| --- | --- | --- | --- |
| `POST` | `/api/auth/register` | public | `201` created user |
| `POST` | `/api/auth/login` | public | `200` token |
| `POST` | `/api/users` | JWT | `201` created user |
| `GET` | `/api/users` | JWT | `200` list of users |
| `GET` | `/api/users/:id` | JWT | `200` user |
| `PATCH` | `/api/users/:id` | JWT | `200` updated user |
| `DELETE` | `/api/users/:id` | JWT | `204` no body |
| `GET` | `/health` | public | `200` |

### Examples

**Create a user**

```http
POST /api/users
Authorization: Bearer <token>

{"name": "Bob", "email": "bob@example.com", "password": "password123"}
```

```http
201 Created

{"id": "6af53a00f3824b8b5e843f75", "name": "Bob", "email": "bob@example.com", "created_at": "2026-11-12T04:01:04.43Z"}
```

**List users**

```http
GET /api/users
Authorization: Bearer <token>
```

```http
200 OK

{
  "users": [
    {"id": "6af53a00f3824b8b5e843f73", "name": "Alice", "email": "alice@example.com", "created_at": "2026-11-12T04:01:04.013Z"},
    {"id": "6af53a00f3824b8b5e843f75", "name": "Bob", "email": "bob@example.com", "created_at": "2026-11-12T04:01:04.43Z"}
  ],
  "total": 2
}
```

**Get a user**

```http
GET /api/users/6af53a00f3824b8b5e843f75
Authorization: Bearer <token>
```

```http
200 OK

{"id": "6af53a00f3824b8b5e843f75", "name": "Bob", "email": "bob@example.com", "created_at": "2026-11-12T04:01:04.43Z"}
```

**Update name or email** (send one or both)

```http
PATCH /api/users/6af53a00f3824b8b5e843f75
Authorization: Bearer <token>

{"name": "Robert", "email": "robert@example.com"}
```

```http
200 OK

{"id": "6af53a00f3824b8b5e843f75", "name": "Robert", "email": "robert@example.com", "created_at": "2026-11-12T04:01:04.43Z"}
```

**Delete a user**

```http
DELETE /api/users/6af53a00f3824b8b5e843f75
Authorization: Bearer <token>
```

```http
204 No Content
```

### Error examples

| Case | Status | Body |
| --- | --- | --- |
| Invalid email format | `400` | `{"error":"must be a valid email address","field":"email"}` |
| Password under 8 characters | `400` | `{"error":"must be at least 8 characters","field":"password"}` |
| `PATCH` with neither name nor email | `400` | `{"error":"name or email is required","field":"body"}` |
| Wrong password or unknown email | `401` | `{"error":"invalid email or password"}` |
| Missing or invalid token | `401` | `{"error":"invalid or expired token"}` |
| Unknown or malformed user ID | `404` | `{"error":"user not found"}` |
| Email already registered | `409` | `{"error":"email is already registered","field":"email"}` |
| Unexpected server error | `500` | `{"error":"internal server error"}` (details are logged, not returned) |

## Background job and logging

- Every request is logged by `middleware.Logger`:
  `2026/11/12 11:01:04 POST /api/auth/register 201 67.58ms`
- `worker.RunUserCounter` runs in its own goroutine and logs
  `total users: N` every 10 seconds. It stops when the application shuts down.

## Graceful shutdown

`main` listens for `SIGINT` and `SIGTERM` with `signal.NotifyContext`. When a
signal arrives it:

1. stops accepting new requests and waits for running ones (`http.Server.Shutdown`, up to `SHUTDOWN_TIMEOUT`),
2. waits for the user count goroutine to stop,
3. disconnects from MongoDB.

## Design decisions and assumptions

- **Unique email is enforced by MongoDB**, with a unique index created at
  startup. Checking "does this email exist?" before inserting would leave a
  race between two requests; the index makes the second insert fail, and the
  repository turns that into `409`.
- **Emails are stored in lowercase and trimmed**, so `Alice@Example.com` and
  `alice@example.com` are the same account. Names are trimmed and must not be blank.
- **Any authenticated user can manage all users.** The challenge does not
  define roles, so there is no admin/owner check. In production I would add roles
  or allow users to change only their own record.
- **Register and `POST /api/users` share the same logic.** Register is public
  so the first user can be created; `POST /api/users` requires a token.
- **Login does not reveal whether an email exists**: wrong password and unknown
  email return the same message, and an unknown email still runs a bcrypt
  comparison so both take about the same time.
- **Password**: 8–72 characters (bcrypt only uses the first 72 bytes). It is
  never returned by the API.
- **Malformed IDs return `404`**, not `400`: from the client's point of view
  there is no user with that ID.
- **`PATCH` only changes the fields that are sent** (`null` or missing means
  "keep"). Password changes are not part of this API.
- **`GET /api/users` has no pagination**, as the challenge asks to list all
  users. For a large collection I would add `limit` and a cursor.
- **`created_at` is stored with millisecond precision**, because that is what
  MongoDB stores; the API returns the same value on create and on read.
- **No refresh tokens or logout.** Tokens are short-lived and stateless.

## Bonus items

| Item | Status |
| --- | --- |
| Docker and docker-compose | Done |
| Interfaces over MongoDB | Done (`repository.UserRepository`, used by tests) |
| Input validation | Done (required, email format, length, blank name) |
| Graceful shutdown | Done |
| gRPC | Not done. I chose to spend the time on tests and the design document. |
| Hexagonal architecture | Partly: the layers map to it. `service` is the application layer, `repository/interface.go` is the port, and the MongoDB repository is the adapter. Business logic does not import the MongoDB driver except for the ID type in `entity`. |

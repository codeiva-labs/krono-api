# krono-api

A REST API for **Krono**, a personal time-tracking app. Users log in, record "activities" (things they spend time on, tagged with a category), and pull back statistics on how their time is split across categories over a day/week/month/year.

Built in Go with [gorilla/mux](https://github.com/gorilla/mux) for routing and [MongoDB](https://www.mongodb.com/) for storage.

## How it works

- **Auth**: Users register with email/password (hashed with bcrypt) or sign in with Google (ID token verified server-side). Both paths issue a JWT (24h expiry, `HS256`, signed with `JWT_SECRET`) that must be sent as `Authorization: Bearer <token>` on every protected route. `AuthMiddleware` (`app/handler/middleware.go`) validates the token and injects `user_id` into the request context for handlers to use. Password reset is OTP-based: a 6-digit code is emailed and stored in `password_resets` with a 1-hour TTL index that auto-expires it in Mongo.
- **Categories**: A fixed set of ~21 "main" categories (Sleep, Work, Sport & Exercise, etc.) is seeded into the `categories` collection on startup if not already present (`app/db/seeder.go`). Users can additionally create their own custom categories, each of which must have a main category as its `parent_id`. Main categories can't be edited or deleted; custom categories can't be deleted while an activity still references them.
- **Activities**: An activity is a time entry — a title/description tied to a category, with a `start_time` and optional `end_time`. `duration` (seconds) is computed server-side whenever an end time is present. Activities can be listed/filtered by date range and category, and each activity response is joined with its category via a MongoDB aggregation `$lookup`.
- **Statistics**: Two read endpoints aggregate activity durations directly in MongoDB (no in-app computation): daily totals broken down by category over a period, and a ranked "most used categories" view with percentage share of total tracked time. Both respect a per-request `X-Timezone` header (IANA name, e.g. `Asia/Jakarta`) so day boundaries are computed in the user's local time rather than UTC.
- **Notifications**: Login and password-reset emails are sent asynchronously (fire-and-forget goroutines) via plain SMTP (`pkg/mail`). If SMTP env vars aren't set, sending is silently skipped — the API doesn't block or fail on missing mail config.

## Architecture

```
main.go                   entrypoint: loads .env, reads config, starts the app
app/
  app.go                  App struct, route table, request/logging middleware
  db/seeder.go            seeds the fixed list of main categories on boot
  handler/                one file per resource; each handler takes (*model.Collections, w, r)
    auth.go               register, login, google auth, password reset
    activity.go            CRUD for activities
    category.go            list/create/edit/delete categories
    stat.go                 daily stats + most-used-categories aggregations
    middleware.go           JWT auth middleware
    health.go               GET /health
  model/                  Mongo document structs + collection setup/indexes
config/config.go          env-driven config, Mongo connection
pkg/
  response/               uniform {code, message, data} JSON envelope
  mail/                   minimal SMTP client (STARTTLS + implicit TLS)
```

Requests flow: `mux.Router` → (optional) `AuthMiddleware` → `app.handleRequest` (injects `*model.Collections`) → handler function → `pkg/response` writes a consistent JSON envelope. There's no service/repository layer — handlers talk to MongoDB collections directly.

## Configuration

All configuration is via environment variables (loaded from `.env` if present):

| Variable | Required | Description |
| --- | --- | --- |
| `MONGO_URI` | yes | MongoDB connection string |
| `MONGO_DB` | yes | Database name |
| `MONGO_CONNECT_TIMEOUT` | no | Connection timeout in seconds (default `10`) |
| `PORT` | no | HTTP port (default `3000`) |
| `JWT_SECRET` | recommended | HMAC secret for signing JWTs (defaults to `dev-secret` if unset — do not use that default in production) |
| `GOOGLE_CLIENT_ID` | for Google sign-in | OAuth client ID used to verify Google ID tokens |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` | for email | SMTP credentials for login/reset notifications; email sending is skipped if unset |

## Running locally

```bash
# fetch modules
go mod tidy

# run (requires MONGO_URI and MONGO_DB to be set, e.g. via .env)
go run main.go
```

```bash
curl -sS http://localhost:3000/health
# => {"code":200,"message":"ok","data":{"status":"ok"}}
```

A `Dockerfile` is included for building a minimal Alpine-based image (multi-stage build, non-root user, built-in `/health` healthcheck).

## API overview

All responses use the envelope `{"code": <int>, "message": <string>, "data": <any>}`. Protected routes require `Authorization: Bearer <jwt>`.

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/health` | no | Liveness check |
| POST | `/auth/register` | no | Create a user (email, password, name) |
| POST | `/auth/login` | no | Email/password login, returns JWT |
| POST | `/auth/authenticate-google` | no | Google ID token login/registration, returns JWT |
| POST | `/auth/request-password-reset` | no | Emails a 6-digit OTP (always returns success, doesn't leak whether the email exists) |
| POST | `/auth/reset-password` | no | Verifies OTP and sets a new password |
| GET | `/categories` | yes | List main + the user's custom categories |
| POST | `/categories/add` | yes | Create a custom category under a main category |
| PUT | `/categories/{category_id}/edit` | yes | Edit a custom category (not main ones) |
| DELETE | `/categories/{category_id}/delete` | yes | Delete a custom category (must be unused by any activity) |
| GET | `/activities` | yes | List activities, filterable by `start`, `end` (RFC3339), `category_id` |
| GET | `/activities/{activity_id}` | yes | Get a single activity |
| POST | `/activities/add` | yes | Create an activity |
| PUT | `/activities/{activity_id}/edit` | yes | Update an activity |
| DELETE | `/activities/{activity_id}/delete` | yes | Delete an activity |
| GET | `/stats/daily?period=week\|month\|year` | yes | Per-day totals by category (honors `X-Timezone` header) |
| GET | `/stats/most-used-categories?period=day\|week\|month\|year` | yes | Ranked categories by tracked time, with percentage share |

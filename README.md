# Graduation Live Reaction System

Production-oriented realtime graduation reactions for anonymous audience phones, authenticated control-room operators, and authorized large-screen displays.

## Architecture

```text
Nuxt 3 (/join/:code, /display/:code, /panel)
       │ REST + one WebSocket endpoint
       ▼
Go HTTP server
  ├─ authenticated REST room/admin API
  ├─ concurrency-safe in-memory room runtimes
  ├─ bounded per-client send queues
  ├─ per-connection token-bucket rate limiting
  ├─ 150 ms reaction aggregation
  ├─ periodic presence and health updates
  └─ 5 s aggregated statistics flush
       │ persistent operations only
       ▼
MySQL 8
```

Reactions never query or write MySQL. Accepted reactions increment in-memory counters and a room buffer. One batch per interval goes only to displays and admins. Aggregate counts are flushed periodically. Students receive state, presence, prompts, and configuration—not other students' reactions.

## Repository

- `apps/web/`: Nuxt 3, Vue 3 Composition API, TypeScript, and Tailwind; deploy this application to Vercel
- `apps/server/`: persistent Go REST/WebSocket service and load generator; deploy this application to a VPS
- `apps/server/migrations/`: embedded, idempotent MySQL schema
- `package.json` and `package-lock.json`: root npm workspace and shared frontend dependency lock
- `go.work`: root Go workspace for server commands

## Prerequisites

- Go 1.26.6+
- Node.js 22+ with npm
- MySQL 8.4+
- A modern browser. Display rendering uses Canvas 2D and `requestAnimationFrame`.

## Local development

1. Start the native MySQL service and create the application database and user:

   ```sql
   CREATE DATABASE graduation
     CHARACTER SET utf8mb4
     COLLATE utf8mb4_0900_ai_ci;
   CREATE USER 'graduation'@'localhost' IDENTIFIED BY 'change-me';
   GRANT ALL PRIVILEGES ON graduation.* TO 'graduation'@'localhost';
   FLUSH PRIVILEGES;
   ```

2. Copy `.env.example` to `.env`, set the same MySQL password in `DATABASE_URL`, and replace the bootstrap password. Admin passwords must contain 12–72 bytes.

3. Install frontend dependencies from the repository root:

   ```bash
   npm install
   ```

4. Start the backend from the repository root:

   ```bash
   npm run dev:server
   ```

   The backend reads the root `.env`, connects to native MySQL, applies all embedded migrations in `apps/server/migrations`, and creates the bootstrap admin only when the `admins` table is empty.

5. Start the frontend in another terminal:

   ```bash
   npm run dev:web
   ```

6. Open `http://localhost:3000/panel` and sign in with `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD`.
7. Create and start a room, authorize a display, then open the generated display link and `/join/{ROOM_CODE}`.

The web application defaults to `http://localhost:8080` and `ws://localhost:8080/ws`. To use different addresses, set `NUXT_PUBLIC_API_BASE` and `NUXT_PUBLIC_WS_URL` in the frontend process environment before running Nuxt.

## Environment variables

| Variable | Purpose | Default |
|---|---|---|
| `DATABASE_URL` | Go MySQL DSN; include `parseTime=true&multiStatements=true` | required |
| `HTTP_ADDR` | Backend listen address | `:8080` |
| `FRONTEND_URL` | Exact allowed browser origin | `http://localhost:3000` |
| `COOKIE_SECURE` | Secure cookie + `SameSite=None` for separate HTTPS origins | `false` |
| `BOOTSTRAP_ADMIN_EMAIL` | First admin email | required on empty DB |
| `BOOTSTRAP_ADMIN_PASSWORD` | First admin password, 12–72 bytes | required on empty DB |
| `REACTION_RATE_LIMIT` | Default accepted reactions/second | `5` |
| `NUXT_PUBLIC_API_BASE` | Browser-visible backend HTTP URL | `http://localhost:8080` |
| `NUXT_PUBLIC_WS_URL` | Browser-visible WebSocket URL | `ws://localhost:8080/ws` |

The display token appears once when an operator authorizes a display. The display stores it locally after removing it from the address bar. Revoke it from the backend API to disconnect that display immediately.

## Routes

- `/join/{room_code}`: anonymous, mobile-first reaction controls
- `/display/{room_code}?token=...`: authorized 16:9 display
- `/panel`: authenticated operations UI

## REST API

Public:

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/api/auth/login` | Creates HTTP-only admin session cookie |
| `POST` | `/api/auth/logout` | Revokes session |
| `GET` | `/api/rooms/:code/public` | Minimal title, status, lock, reaction config |
| `GET` | `/health` | Process liveness |

Authenticated admin session required:

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/api/auth/me` | Validate session |
| `GET, POST` | `/api/rooms` | List/create rooms |
| `GET, PATCH, DELETE` | `/api/rooms/:code` | Inspect/edit/delete room |
| `POST` | `/api/rooms/:code/start` | `WAITING → ACTIVE` |
| `POST` | `/api/rooms/:code/pause` | `ACTIVE → PAUSED` |
| `POST` | `/api/rooms/:code/resume` | `PAUSED → ACTIVE` |
| `POST` | `/api/rooms/:code/end` | Any non-ended state → `ENDED` |
| `POST` | `/api/rooms/:code/lock` | Reject new student joins |
| `POST` | `/api/rooms/:code/unlock` | Permit student joins |
| `GET` | `/api/rooms/:code/stats` | Runtime totals and persisted time buckets |
| `GET, POST` | `/api/rooms/:code/displays` | List/authorize room displays |
| `DELETE` | `/api/rooms/:code/displays/:id` | Revoke and disconnect display |
| `GET` | `/api/system` | Runtime metrics and operational events |

Room codes are uppercase alphanumeric, 4–12 characters. Omitting a code on creation generates a six-character human-readable code.

## WebSocket protocol

Endpoint: `/ws`. Messages are UTF-8 JSON, maximum 4 KiB. The first message **must** be `join`. The server binds role and room to that connection; reaction and admin-control messages cannot choose or affect another room. Browser origins must match `FRONTEND_URL`. Non-browser clients may omit `Origin`.

### Client → server

Student join:

```json
{"type":"join","role":"student","room":"GRAD26","client_id":"c81ea1c3-..."}
```

Display join (room-specific token):

```json
{"type":"join","role":"display","room":"GRAD26","token":"display-token"}
```

Admin join. Authentication comes from the HTTP-only session cookie; no credential is exposed in JSON:

```json
{"type":"join","role":"admin","room":"GRAD26"}
```

Common messages:

```json
{"type":"reaction","emoji":"🔥"}
{"type":"ping","sent_at":1770000000000}
```

Admin messages:

```json
{"type":"pause"}
{"type":"resume"}
{"type":"clear_display"}
{"type":"lock_room"}
{"type":"unlock_room"}
{"type":"disconnect_all"}
{"type":"set_display_mode","mode":"COUNTDOWN","duration":10}
{"type":"set_display_background","background":"transparent"}
{"type":"announcement","message":"CONGRATULATIONS CLASS OF 2026!","duration":7}
{"type":"prompt","message":"MAKE SOME NOISE! 🔥"}
{"type":"reaction_config","reactions":["❤️","🔥","👏","🎓","🥳","✨"]}
```

Allowed display modes: `REACTIONS`, `ANNOUNCEMENT`, `COUNTDOWN`, `CELEBRATION`, `QR`, `BLANK`.

### Server → client

`joined` is a complete resynchronization snapshot after every join/reconnect:

```json
{
  "type":"joined",
  "room":{"code":"GRAD26","title":"Graduation 2026","status":"ACTIVE","locked":false,"background":"#09090b"},
  "presence":{"online":647,"students":643,"admins":2,"displays":2,"unique_clients":642},
  "reaction_config":["❤️","🔥","😂","😭","👏","🎓"],
  "mode":"REACTIONS",
  "background":"#09090b",
  "message":"",
  "countdown_ends_at":0
}
```

Other server events:

```json
{"type":"reaction_batch","reactions":{"❤️":73,"🔥":120,"👏":182}}
{"type":"presence","presence":{"online":647,"students":643,"admins":2,"displays":2,"unique_clients":642},"displays":[]}
{"type":"room_state","room":{"code":"GRAD26","title":"Graduation 2026","status":"PAUSED","locked":false}}
{"type":"reaction_config","reaction_config":["❤️","🔥","👏","🎓","🥳","✨"]}
{"type":"display_mode","mode":"CELEBRATION"}
{"type":"display_background","background":"#10243a"}
{"type":"announcement","message":"CONGRATULATIONS! 🎓","duration":7}
{"type":"prompt","message":"Give them an applause! 👏"}
{"type":"clear_display"}
{"type":"display_health","displays":[{"id":"display-1","name":"Main Videotron","online":true,"latency_ms":12,"last_heartbeat":1770000000000}]}
{"type":"pong","sent_at":1770000000000,"server_time":1770000000012}
{"type":"system","code":"shutdown","message":"Server is restarting"}
```

Display backgrounds are stored per room. Accepted values are `transparent` or a six-digit CSS hex color such as `#10243a`. Announcements accept a duration from 5–10 seconds; the server clamps out-of-range values and displays restore their prior mode when the timer ends.

The server also uses WebSocket protocol ping frames every 20 seconds and expires dead connections after 45 seconds. Browser clients reconnect with capped exponential backoff: 500 ms, 1 s, 2 s, 4 s, then 5 s.

## Runtime and performance invariants

- One goroutine reading and one writing per connection; no goroutine per reaction.
- A room lock protects only maps/counters. Socket writes happen after lock release.
- Each connection has a 64-message bounded queue. A persistently slow consumer is disconnected.
- Per-connection token bucket defaults to 5 reactions/second. Excess is silently counted and dropped.
- Reaction batches flush every 150 ms only to display/admin roles.
- Presence flushes every 3 seconds.
- Statistics flush every 5 seconds in aggregate; failed flushes return counts to memory.
- Display Canvas rendering enforces a hard limit of 100 live emoji particles and scales representative particles by the square root of each batch size.

## Load testing

Create and start the target room first, then run:

```bash
npm run loadtest -- --url ws://localhost:8080/ws --room GRAD26 --users 1000 --rate 2 --duration 60s
```

The report includes connected clients, connection failures, sent reactions, message errors, elapsed time, and aggregate send throughput. Progress through 100, 500, 1,000, then 2,000 users. Watch `/panel` → **System** and host CPU/RAM. On Linux, raise file descriptors before 2,000 connections (for example `ulimit -n 10000`). Do not interpret client-side send throughput as end-to-end display latency; observe batch arrival timing on an authorized display when profiling.

## Deployment

### Nuxt web application on Vercel

The Go WebSocket server is intentionally **not** a Vercel application. It holds active rooms and connections in process memory and requires a persistent server. Deploy only `apps/web` to Vercel and keep `apps/server` on a VPS.

1. Import this Git repository as a Vercel project.
2. Set **Root Directory** to `apps/web`.
3. Keep the detected **Nuxt.js** framework and default `npm run build` command. `apps/web/vercel.json` declares the framework; Nuxt selects the Vercel Nitro preset in Vercel's build environment.
4. Add `NUXT_PUBLIC_API_BASE=https://api.example.com` and `NUXT_PUBLIC_WS_URL=wss://api.example.com/ws` to the Vercel project's Production, Preview, and Development environments as appropriate.
5. Deploy. The root npm workspace and lockfile provide deterministic dependency installation.

The frontend and API must both use HTTPS in production. Prefer same-site custom domains such as `app.example.com` and `api.example.com`; unrelated domains may cause browsers to block the admin session cookie.

### Go backend on a VPS

Build the server from the repository root:

```bash
mkdir -p bin
go build -trimpath -ldflags="-s -w" -o ./bin/graduation-server ./apps/server/cmd/server
```

Run `bin/graduation-server` under the host service manager with production environment variables supplied by that manager.

- Run one persistent Go process behind a reverse proxy that supports WebSocket upgrades and uses timeouts longer than 45 seconds.
- Terminate TLS at the proxy and use `wss://`. Set `COOKIE_SECURE=true` in production.
- Set `FRONTEND_URL` to the exact Vercel production origin.
- Keep a single backend instance for V1 because active room state is process-local. Horizontal replicas require deliberate room affinity or shared realtime infrastructure and are not part of this design.
- Allocate at least 2 vCPU and 4 GB RAM, set a high file-descriptor limit, and load test on the target VPS.
- Back up MySQL. It contains room configuration, credentials, sessions, display authorizations, events, and aggregate analytics—not raw reactions.
- SIGINT/SIGTERM stops HTTP acceptance, flushes pending statistics, sends a shutdown event, closes WebSockets, and closes MySQL.
- Use external TLS, process supervision, MySQL backups, and host-level CPU/network monitoring. `/health` is suitable for liveness; `/api/system` is authenticated operational telemetry.

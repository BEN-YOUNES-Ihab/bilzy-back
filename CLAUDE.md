# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Stack & commands

Go 1.22+, [chi](https://github.com/go-chi/chi) router, [pgx/v5](https://github.com/jackc/pgx) directly (no ORM, no sqlc — see "Data layer" below), [goose](https://github.com/pressly/goose) for migrations, [Firebase Admin SDK](https://firebase.google.com/docs/admin/setup) for token verification, stdlib `log/slog` for logs.

```
make dev            # go run ./cmd/server
make build          # compile to bin/server
make migrate-up     # apply migrations against $DATABASE_URL
make migrate-status
make tidy           # go mod tidy
make test           # go test ./...
```

Migrations and the `goose` CLI are external — install once with `go install github.com/pressly/goose/v3/cmd/goose@latest`. The Makefile sources `.env` so `DATABASE_URL` is read automatically.

`.env` is required: `DATABASE_URL` (Neon Postgres), `FIREBASE_PROJECT_ID`, and either `GOOGLE_APPLICATION_CREDENTIALS` (path to service-account JSON) or rely on Application Default Credentials. `cmd/server/main.go` exits at startup if `DATABASE_URL` or `FIREBASE_PROJECT_ID` is missing.

## Project layout

```
cmd/server/main.go            # wiring: config → db pool → firebase → router
internal/
  config/                     # env loading
  firebase/                   # admin SDK init + ID token verification
  db/
    pool.go                   # pgxpool factory
    migrations/0001_init.sql  # the entire schema, in one goose-managed file
    store/                    # *.go — typed methods, one file per entity
  domain/                     # request/response DTOs (shared with HTTP layer)
  http/
    router.go                 # chi router; mounts handlers
    middleware/               # cors, logger, recover
    middleware_auth.go        # firebase token verify + bootstrap-on-first-request
    errors.go                 # APIError + WriteJSON / WriteError
    context.go                # WithUserID / MustUserID
    profiles.go shops.go categories.go closings.go
```

## Auth model

The frontend obtains a Firebase ID token (sign-in via email/password, Google, or Apple) and sends `Authorization: Bearer <token>` on every request. `internal/http/middleware_auth.go`:

1. Extracts the bearer token.
2. Verifies it with the Firebase Admin SDK (`firebase.Client.Verify`).
3. Looks up `profiles` by `firebase_uid`. **First-login bootstrap**: if no row exists, creates the profile and seeds 10 default categories in a single transaction (replaces the old Supabase `on_auth_user_created` triggers).
4. Puts the internal `profiles.id` (uuid) on the request context as `userID`.
5. If `email` in the token differs from `profiles.email`, runs a cheap `UPDATE` to sync (covers the `verifyBeforeUpdateEmail` flow).

Handlers retrieve the user via `MustUserID(ctx)` — never trust client-supplied user IDs in request bodies. Ownership is enforced in handler / store code (every shop/category/closing query scopes by `owner_id = $1`).

## Data layer

Hand-rolled `pgx` SQL inside `internal/db/store/*.go`. There is no ORM and no codegen — just a `Store` struct with typed methods returning domain types. The trade-off vs sqlc is: easier to read and modify, no extra build step; you write the SQL, the scan, and the mapping yourself.

Conventions:
- Reads return `(*T, error)` for single rows (`(nil, nil)` for not-found) or `([]T, error)` for collections.
- Writes that span multiple statements use `pool.BeginTx` with `defer tx.Rollback`.
- The store never reads ambient state — pass the authenticated `ownerID` explicitly. This keeps tests deterministic.
- `store.IsUniqueViolation(err)` and `store.IsNoRows(err)` map common pgx errors; handlers turn these into `409 duplicate` / `404 not_found`.

## Migrations

`internal/db/migrations/000N_*.sql`, run by `goose`. Every statement that's not a single-line one must be wrapped in `-- +goose StatementBegin / StatementEnd` because of the `$$` blocks in functions and triggers.

**No RLS.** Authorization is enforced in Go. Don't add `enable row level security` or `create policy` — it has no Supabase counterpart on Neon, would require `SET LOCAL` ceremony to thread the user ID, and complicates testing.

**No `auth.uid()` defaults.** All `owner_id` / `entered_by` columns are filled by handlers using the context user ID.

When adding a new table, in the same migration: add the table, add `references profiles(id) on delete cascade` for the owner column, add the relevant index. Test with `make migrate-up` then `make migrate-down` then `make migrate-up`.

## Domain invariants (load-bearing across the contract)

The frontend depends on these — don't change without updating the client at the same time:

- **`'cash'` revenue slug** is referenced in the client's float-diff math (`diff = floatClose - (floatOpen + cashRevenue - expenses)`). Keep it in the system seeds in `seed_default_categories()`.
- **Five `categories.color` slots**: `brand | accent | warn | info | ink3`. Adding a sixth requires updating the DB CHECK constraint, the client's `CategoryColor` type, and the chart-hex map together.
- **`closings(shop_id, date)` UNIQUE**: closings are upserted on this key. `SaveClosing` deletes child `closing_revenues` + `closing_expenses` and reinserts in one tx — re-saving the same date never accumulates.
- **Slug format**: `categories.slug` matches `^[a-z0-9_]+$`, length 1–64. Custom (user-created) slugs are prefixed `cust_` followed by 8 random alphanumerics. System seeds use stable slugs (`cash`, `card`, `supplier`, …).
- **Money is `numeric` in Postgres, `float64` over the wire.** pgx scans `numeric` cleanly into Go floats; the client coerces with `Number(x)` because JS doesn't distinguish.
- **Dates** in API params and JSON bodies are `YYYY-MM-DD` strings (the device's local calendar). Use `domain.ParseDate` / `FormatDate`. Never accept or emit timezone-shifted ISO timestamps for `closings.date`.

## Error contract

`internal/http/errors.go` defines `APIError { code, message }` with HTTP status. The frontend's `friendlyError(err, tr)` switches on `err.code` to pick a translation key, so the codes in `ErrorCode` (`unauthorized`, `forbidden`, `not_found`, `validation`, `duplicate`, `invalid_credentials`, `internal`) are part of the public contract — don't rename without updating the frontend.

Use `WriteError(w, err)` to emit any `APIError`; raw `error` values become 500 with a generic message and the original is logged via slog.

## Rules

- Don't introduce ORMs or query builders. Inline pgx + a small `Store` is the design; reach for it before adding a dep.
- Don't add per-request DB writes outside `middleware_auth.go`'s deliberate `SyncProfileEmail`. Reads are cheap; writes accumulate.
- Don't trust request bodies for `owner_id`, `entered_by`, or any identity. Always read from `MustUserID(ctx)`.
- Don't expose internal user uuids to the frontend gratuitously. The `/api/me` debug endpoint is the exception; everything else returns scoped data without echoing the caller's ID.
- Validate at the handler boundary (kind / color enums, uuid parsing, date parsing). The store assumes valid inputs.
- The five-slot `CategoryColor` and the `'cash'` slug are co-located contracts with the frontend — touch them only as part of a coordinated change.

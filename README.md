# mobile_server

Standalone backend for the mobile application.

## Purpose

This repository runs the mobile API without the Telegram bot process.

Main flow:

- Mobile app -> `mobile_server` -> ERPNext API

Current transaction rules:

- supplier dispatch creates `Purchase Receipt`
- werka customer issue creates submitted `Delivery Note`
- customer confirm updates the original `Delivery Note`
- customer reject creates a real return `Delivery Note` against the original one
- customer reject is not treated as a UI-only status change

## Run

```bash
make run
```

The server starts on `:8081` by default and loads `.env` automatically.

Health check:

```bash
curl http://127.0.0.1:8081/healthz
```

Expected response:

```json
{"ok":true}
```

## Stop

```bash
make stop
```

## Test

```bash
go test ./...
```

## Notes

- `ERPNext` source code is not edited from this repo.
- Firebase service account JSON is local-only and should not be committed.
- This repo is the primary backend target for mobile work.
- `Werka` auth is code-driven and should not be blocked by phone format drift.

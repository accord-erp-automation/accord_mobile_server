# Accord Mobile Server

Accord Mobile Server is the standalone Go backend for the mobile workflow on top of ERPNext. It runs independently from the Telegram bot and serves the mobile app directly.

Main flow:

`mobile_app -> mobile_server -> ERPNext`

## Overview

This backend is responsible for:

- authenticating `Supplier`, `Werka`, `Customer`, and `Admin`
- translating mobile actions into ERPNext document operations
- maintaining mobile-facing workflow state
- managing push tokens and FCM notifications
- keeping ERP comments and ERP custom fields aligned with business actions

The server is intentionally the main business layer for mobile behavior. The app should stay relatively thin and defer business truth to this backend and ERPNext.

## Current Business Rules

These rules reflect the current intended behavior:

- supplier dispatch creates a `Purchase Receipt`
- werka customer issue creates and submits a `Delivery Note`
- werka single-submit now uses the explicit app payload directly on the hot path
- werka batch-submit is available through `/v1/mobile/werka/customer-issue/batch-create`
- customer confirm updates the original `Delivery Note`
- customer reject creates and submits a real return `Delivery Note`
- customer reject is not treated as a UI-only status flip
- comments are discussion/audit history only
- ERP fields remain the source of business state
- Werka auth is code-driven and should not be blocked by phone format drift

## Delivery Note Semantics

Customer delivery state is tracked on top of ERPNext `Delivery Note`.

Current fields used:

- `accord_flow_state`
- `accord_customer_state`
- `accord_customer_reason`
- `accord_delivery_actor`
- `accord_ui_status`

Current state expectations:

- `accord_flow_state`
  - `1` = submitted
- `accord_customer_state`
  - `1` = pending
  - `2` = rejected
  - `3` = confirmed
- `accord_ui_status`
  - `pending`
  - `confirm`
  - `partial`
  - `rejected`

Important rule:

- `confirm` must not create a return document
- `reject` must create a real return `Delivery Note` with `is_return = 1` and `return_against = <original DN>`

## Main Transaction Flows

### Supplier

- login
- fetch summary/history/items
- create dispatch
- backend writes `Purchase Receipt`

Relevant backend logic:

- [service.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go)
- [server.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go)

### Werka

- login
- fetch summary/history/pending
- create single customer issue
- create batch customer issue
- backend writes submitted `Delivery Note`
- direct DB read is only for supported picker/read flows
- create/submit flows still write through ERPNext HTTP APIs

Relevant backend logic:

- [service.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go)
- [server.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go)

### Customer

- fetch summary/history/detail
- approve or reject an existing `Delivery Note`

Current behavior:

- approve updates original DN state
- reject creates a real return DN and updates original DN state

Relevant backend logic:

- [service.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go)
- [server.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go)

### Admin

- settings
- supplier/customer management
- item assignment
- activity feed

## API Surface

The HTTP layer is intentionally thin and delegates to `ERPAuthenticator`.

Core entrypoint:

- [main.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/cmd/core/main.go)

HTTP router:

- [server.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go)

Business layer:

- [service.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go)

ERP adapter:

- [client.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/client.go)
- [purchase_receipt.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/purchase_receipt.go)
- [delivery_note.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/delivery_note.go)
- [customer.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/customer.go)

## Custom ERP Field Handling

This backend includes fallback logic to ensure delivery-note workflow fields exist in ERPNext.

Relevant file:

- [delivery_note.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/delivery_note.go)

Related ERP custom app:

- `/home/wikki/storage/local.git/erpnext_n1/erp/apps/accord_state_core`

The ERP app is still the cleaner long-term home for field management, but backend fallback exists so mobile operations do not fail when field setup drifts.

## Push Notifications

The backend stores mobile push tokens and sends FCM notifications for role-specific events.

Relevant files:

- [push_token_store.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/core/push_token_store.go)
- [fcm.go](/home/wikki/storage/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/fcm.go)

Operational notes:

- stale FCM tokens are dropped automatically
- push routing is role/ref based
- request-level logging was added for `Delivery Note` customer issue creation

## Performance And Direct Read Notes

- `ERP_DIRECT_READ_ENABLED=1` only affects supported read-heavy Werka picker and summary flows
- `Delivery Note` create and submit flows still go through ERPNext HTTP APIs
- single Werka customer issue submit now uses the selected `customer_ref`, `item_code`, and `qty` directly
- batch Werka customer issue submit is exposed through `/v1/mobile/werka/customer-issue/batch-create`
- batch lines are processed in parallel in the backend so multi-item submit stays close to single-item submit latency

## Runtime Files

By default the backend uses local JSON files for mobile state:

- `data/mobile_profile_prefs.json`
- `data/mobile_admin_suppliers.json`
- `data/mobile_push_tokens.json`
- `data/mobile_sessions.json`

These are suitable for local/single-instance operation, but not a strong multi-instance persistence strategy.

Session behavior:

- login sessions survive backend restart when `data/mobile_sessions.json` is preserved
- default session TTL is `720` hours (`30` days)
- set `MOBILE_API_SESSION_TTL_HOURS=0` to disable expiry

## Run

```bash
make run
```

The server starts on `:8081` by default and loads `.env` automatically.

Werka AI image search now runs through `mobile_server`, not the mobile app
binary. Set these env vars on the server when you want the scan button to work:

- `GEMINI_API_KEY=<your gemini api key>`
- `GEMINI_VISION_MODEL=gemini-flash-lite-latest` (optional)

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

## Debugging Notes

When debugging mobile delivery flows, check these in order:

1. `mobile_server/.core.log`
2. whether `/v1/mobile/werka/customer-issue/create` was hit
3. whether ERPNext received a new `Delivery Note`
4. whether customer response hit `/v1/mobile/customer/respond`
5. whether a return `Delivery Note` was created only on reject

Useful live checks:

- local backend: `http://127.0.0.1:8081/healthz`
- public backend: `https://core.wspace.sbs/healthz`
- ERP endpoint base: value from `.env` `ERP_URL`

## Notes

- `ERPNext` core source is not edited from this repo
- Firebase service account JSON is local-only and should not be committed
- this repo is the primary backend target for mobile work
- if business behavior changes, update this README and commit it

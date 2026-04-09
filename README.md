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

- [service.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go#L1635)
- [server.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go#L793)

### Werka

- login
- fetch summary/history/pending
- create customer issue
- backend writes submitted `Delivery Note`

Relevant backend logic:

- [service.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go#L1217)
- [server.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go#L940)

### Customer

- fetch summary/history/detail
- approve or reject an existing `Delivery Note`

Current behavior:

- approve updates original DN state
- reject creates a real return DN and updates original DN state

Relevant backend logic:

- [service.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go#L1475)
- [server.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go#L638)

### Admin

- settings
- supplier/customer management
- item assignment
- activity feed

## API Surface

The HTTP layer is intentionally thin and delegates to `ERPAuthenticator`.

Core entrypoint:

- [main.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/cmd/core/main.go#L15)

HTTP router:

- [server.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/server.go#L32)

Business layer:

- [service.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/core/service.go#L48)

ERP adapter:

- [client.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/client.go#L1)
- [purchase_receipt.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/purchase_receipt.go#L1)
- [delivery_note.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/delivery_note.go#L1)
- [customer.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/customer.go#L1)

## Custom ERP Field Handling

This backend includes fallback logic to ensure delivery-note workflow fields exist in ERPNext.

Relevant file:

- [delivery_note.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/erpnext/delivery_note.go#L139)

Related ERP custom app:

- `/home/wikki/local.git/erpnext_n1/erp/apps/accord_state_core`

The ERP app is still the cleaner long-term home for field management, but backend fallback exists so mobile operations do not fail when field setup drifts.

## Push Notifications

The backend stores mobile push tokens and sends FCM notifications for role-specific events.

Relevant files:

- [push_token_store.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/core/push_token_store.go#L1)
- [fcm.go](/home/wikki/local.git/erpnext_stock_telegram/mobile_server/internal/mobileapi/fcm.go#L1)

Operational notes:

- stale FCM tokens are dropped automatically
- push routing is role/ref based
- request-level logging was added for `Delivery Note` customer issue creation

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

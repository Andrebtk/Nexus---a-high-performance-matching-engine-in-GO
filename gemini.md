# Nexus — High-Performance Matching Engine — Identity

## What this project is
A full-stack exchange simulator: a Go limit-order-book matching engine (FIFO price-time priority)
with REST + WebSocket APIs, PostgreSQL persistence, a live price oracle (Twelve Data), background
market-maker bots, and a React/Redux trading UI.

## Tech Stack
- Backend: Go (module: Nexus), REST + WebSocket, PostgreSQL
- Frontend: React 19, React Router, Recharts (D3 under the hood), Vite 8
- Diagrams: archi.py (Graphviz) generates system_architecture.{dot,svg,png}

## Project Structure
cmd/api                    → main.go — service wiring, HTTP/WS server bootstrap
internal/
  api/                     → handlers.go, auth_handlers.go (REST + WS handlers)
  database/                → database.go (Postgres connection/pool)
  engine/                  → orderbook.go, limit.go, order.go, order_queue.go, exchange.go — matching core
  models/                  → user.go, transaction.go, postgres_user.go
  oracle/                  → oracle.go — Twelve Data live price feed
  services/                → order_service.go, user_service.go, transaction_service.go,
                              cost_basis_service.go, profit_loss_service.go, postgres_user_service.go
nexus-ui/
  src/components/          → trading UI components
  src/context/             → React context providers
  public/                  → static assets

## Domain Vocabulary
- OrderBook = bidPrices/askPrices (slices) + per-price Limit → OrderQueue (FIFO)
- Exchange.RouteOrder (internal/engine/exchange.go) = single entry point that matches orders and settles balances
- Oracle = external live price feed consumed by market-maker bots
- Cost-basis / P&L services = FIFO realized P&L accounting per user

## Naming Conventions
- Go: standard style — exported PascalCase, unexported camelCase
- React components: PascalCase, functional only, named exports
- Redux slices (if/when added): feature-based file names, Redux Toolkit only

## Key File References (read only when needed — don't preload)
- Matching core: internal/engine/{orderbook.go, exchange.go, limit.go, order_queue.go}
- Entry point / wiring: cmd/api/main.go
- Diagram source of truth: archi.py (NOT the generated .dot/.svg/.png)

## Known Standing Issues (see matching-engine-audit skill for full detail)
1. O(N) slice-based price-level removal in orderbook.go
2. Hardcoded fallback user balance in exchange.go bypasses real validation
3. Synchronous Postgres writes inside RouteOrder (hot path)
4. Mixed float64/uint64 for money — precision risk
5. Hardcoded Twelve Data API key in cmd/api/main.go
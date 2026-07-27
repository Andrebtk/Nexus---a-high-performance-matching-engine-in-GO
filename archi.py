#!/usr/bin/env python3
"""
generate_architecture.py

Generates a detailed system architecture diagram for the Nexus Exchange
project (Go matching engine + React frontend + PostgreSQL).

Requirements:
    pip install graphviz
    (and the system Graphviz binary: apt install graphviz / brew install graphviz)

Usage:
    python generate_architecture.py
    -> writes nexus_architecture.svg and nexus_architecture.png in the
       current directory, plus the raw nexus_architecture.dot source.

Edit the NODES / EDGES / DB_SCHEMA dictionaries below whenever the
codebase changes — this is meant to stay in sync with the project,
not be a one-off snapshot.
"""

from graphviz import Digraph

# ---------------------------------------------------------------------------
# Theme (matches the app's dark "TradingView/Binance" palette)
# ---------------------------------------------------------------------------
BG = "#0b0e11"
PANEL = "#1e2127"
BORDER = "#2b3139"
TEXT = "#eaecef"

COLOR_FRONTEND = "#2962ff"   # blue
COLOR_AUTH = "#0ecb81"       # green
COLOR_API = "#f6465d"        # red
COLOR_ENGINE = "#0ecb81"     # green
COLOR_SERVICE = "#2962ff"    # blue
COLOR_DB = "#4a5568"         # slate
COLOR_EXTERNAL = "#848e9c"   # grey


def base_graph() -> Digraph:
    g = Digraph("NexusArchitecture", format="svg")
    g.attr(rankdir="LR", bgcolor=BG, fontname="Arial", splines="spline")
    g.attr(
        "node",
        fontname="Arial",
        fontsize="12",
        shape="box",
        style="rounded,filled",
        fillcolor=PANEL,
        fontcolor=TEXT,
        color=BORDER,
    )
    g.attr("edge", fontname="Arial", fontsize="10", color=COLOR_EXTERNAL, arrowsize="0.7")
    return g


def node(g, name, label, color, shape="box"):
    g.node(name, label=label, fillcolor=color, shape=shape)


def cluster(g, name, label):
    c = Digraph(name=f"cluster_{name}")
    c.attr(label=label, style="filled", fillcolor=PANEL, fontcolor=TEXT)
    return c


def main():
    g = base_graph()

    # -- Frontend -----------------------------------------------------------
    frontend = cluster(g, "frontend", "Frontend (React / Vite)")
    node(frontend, "react_app", "React Application\n(AppWithAuth.jsx)", COLOR_FRONTEND, "component")
    node(frontend, "auth_context", "AuthContext\n(context/AuthContext.jsx)", COLOR_AUTH, "component")
    node(frontend, "trading_ui", "Trading Interface\n(order book, chart, order form)", COLOR_FRONTEND, "component")
    node(frontend, "profile_page", "Profile Page\n(components/ProfilePage.jsx)", COLOR_FRONTEND, "component")
    node(frontend, "auth_modal", "Auth Modal\n(components/Auth.jsx)", COLOR_AUTH, "component")
    frontend.edge("react_app", "auth_context", label="auth state")
    frontend.edge("react_app", "trading_ui", label="renders")
    frontend.edge("react_app", "profile_page", label="routes to /profile")
    frontend.edge("react_app", "auth_modal", label="shows modal")
    g.subgraph(frontend)

    # -- API layer ------------------------------------------------------------
    api = cluster(g, "api", "API Layer (Gin)")
    node(api, "api_server", "API Server\n(cmd/api/main.go)", COLOR_API, "component")
    node(api, "auth_handlers", "Auth Handlers\n(internal/api/auth_handlers.go)", COLOR_API, "component")
    node(api, "api_handlers", "API Handlers\n(internal/api/handlers.go)", COLOR_API, "component")
    node(api, "jwt_middleware", "JWT Middleware\n(auth check on protected routes)", COLOR_API, "component")
    api.edge("api_server", "auth_handlers", label="/auth/*")
    api.edge("api_server", "api_handlers", label="/order /book /profit-loss ...")
    api.edge("api_handlers", "jwt_middleware", label="protected routes", dir="back")
    g.subgraph(api)

    # -- Core matching engine -------------------------------------------------
    engine = cluster(g, "engine", "Core Matching Engine")
    node(engine, "exchange", "Exchange\n(internal/engine/exchange.go)", COLOR_ENGINE, "component")
    node(engine, "orderbook", "OrderBook\n(bids/asks price levels)", COLOR_ENGINE, "component")
    node(engine, "matching", "Matching Logic\n(matchBuy / matchSell)", COLOR_ENGINE, "component")
    node(engine, "limit_queue", "Limit + OrderQueue\n(doubly-linked list per price level)", COLOR_ENGINE, "component")
    engine.edge("exchange", "orderbook", label="manages per-symbol books")
    engine.edge("orderbook", "matching", label="delegates fills")
    engine.edge("orderbook", "limit_queue", label="price-time priority")
    g.subgraph(engine)

    # -- Services -------------------------------------------------------------
    services = cluster(g, "services", "Services")
    node(services, "user_service", "UserService\n(in-memory, system_bot)", COLOR_SERVICE, "component")
    node(services, "pg_user_service", "PostgresUserService\n(auth, balance, ownership)", COLOR_SERVICE, "component")
    node(services, "order_service", "OrderService\n(order persistence)", COLOR_SERVICE, "component")
    node(services, "transaction_service", "TransactionService\n(ledger entries)", COLOR_SERVICE, "component")
    node(services, "profit_loss_service", "ProfitLossService\n(aggregate P&L)", COLOR_SERVICE, "component")
    node(services, "cost_basis_service", "CostBasisService\n(avg-cost, realized P&L)", COLOR_SERVICE, "component")
    g.subgraph(services)

    # -- Database (with schema detail) ----------------------------------------
    db = cluster(g, "db", "PostgreSQL")
    node(db, "postgres", "PostgreSQL Database", COLOR_DB, "cylinder")
    g.subgraph(db)

    schema = cluster(g, "db_schema", "DB Schema Detail")
    schema.attr(rankdir="TB")
    node(
        schema, "tbl_users",
        "users\n────────────\nid PK\nusername\nemail\npassword_hash\nbalance\nprofit / loss\n"
        "stock_ownership (json)\ncreated_at / updated_at",
        COLOR_DB,
    )
    node(
        schema, "tbl_orders",
        "orders\n────────────\nid PK\nuser_id FK\nsymbol\norder_type\nquantity\nprice\n"
        "status\ncreated_at / updated_at",
        COLOR_DB,
    )
    node(
        schema, "tbl_transactions",
        "transactions\n────────────\nid PK\nuser_id FK\norder_id\namount\ntype\ntimestamp",
        COLOR_DB,
    )
    node(
        schema, "tbl_cost_basis",
        "cost_basis\n────────────\nuser_id PK/FK\nsymbol PK\nquantity\ntotal_cost",
        COLOR_DB,
    )
    schema.edge("tbl_orders", "tbl_users", label="user_id", style="dashed")
    schema.edge("tbl_transactions", "tbl_users", label="user_id", style="dashed")
    schema.edge("tbl_cost_basis", "tbl_users", label="user_id", style="dashed")
    g.subgraph(schema)

    # -- External services -----------------------------------------------------
    external = cluster(g, "external", "External")
    node(external, "price_oracle", "Price Oracle\n(internal/oracle/oracle.go)", COLOR_EXTERNAL, "component")
    node(external, "twelvedata", "TwelveData API\n(live market prices)", COLOR_EXTERNAL, "ellipse")
    node(external, "market_bots", "Market Maker Bots\n(AAPL, MSFT, NVDA, TSLA)", COLOR_EXTERNAL, "component")
    node(external, "users", "Traders / Users", COLOR_EXTERNAL, "ellipse")
    external.edge("price_oracle", "twelvedata", label="polls prices")
    g.subgraph(external)

    # -- Cross-cluster edges (the actual data flow) -----------------------------
    g.edge("users", "react_app", label="uses web UI", color=COLOR_EXTERNAL)
    g.edge("react_app", "api_server", label="HTTP/REST\n(GET/POST)", color=COLOR_API, dir="both")
    g.edge("trading_ui", "api_server", label="GET /book\nPOST /order", color=COLOR_FRONTEND)
    g.edge("auth_modal", "auth_handlers", label="POST /auth/login\nPOST /auth/register", color=COLOR_AUTH)
    g.edge("profile_page", "api_handlers", label="GET /orders/*\nGET /auth/stock-ownership", color=COLOR_FRONTEND)

    g.edge("api_handlers", "exchange", label="RouteOrder()", color=COLOR_ENGINE)
    g.edge("api_handlers", "pg_user_service", label="auth / balance", color=COLOR_SERVICE)
    g.edge("api_handlers", "order_service", label="persist orders", color=COLOR_SERVICE)

    g.edge("exchange", "user_service", color=COLOR_SERVICE)
    g.edge("exchange", "pg_user_service", label="settle balance", color=COLOR_SERVICE)
    g.edge("exchange", "transaction_service", label="record trade", color=COLOR_SERVICE)
    g.edge("exchange", "cost_basis_service", label="avg cost / realized P&L", color=COLOR_SERVICE)
    g.edge("exchange", "profit_loss_service", label="update P&L", color=COLOR_SERVICE)
    g.edge("exchange", "order_service", label="mark completed", color=COLOR_SERVICE)

    g.edge("pg_user_service", "postgres", color=COLOR_SERVICE)
    g.edge("order_service", "postgres", color=COLOR_SERVICE)
    g.edge("transaction_service", "postgres", color=COLOR_SERVICE)
    g.edge("cost_basis_service", "postgres", color=COLOR_SERVICE)
    g.edge("postgres", "tbl_users", style="invis")

    g.edge("price_oracle", "exchange", label="live reference prices", color=COLOR_EXTERNAL)
    g.edge("market_bots", "exchange", label="generated orders", color=COLOR_EXTERNAL)

    return g


if __name__ == "__main__":
    graph = main()

    # Save the raw DOT source explicitly (independent of render() cleanup behavior)
    with open("nexus_architecture.dot", "w") as f:
        f.write(graph.source)

    graph.format = "svg"
    graph.render("nexus_architecture", cleanup=True)

    graph.format = "png"
    graph.render("nexus_architecture", cleanup=True)

    print("Wrote nexus_architecture.svg, nexus_architecture.png, and nexus_architecture.dot (source)")
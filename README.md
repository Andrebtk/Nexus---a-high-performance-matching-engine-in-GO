# Nexus - High Performance Matching Engine

A modern trading platform with a high-performance matching engine built in Go and a React frontend.

## Features

### Core Trading Engine
- **Limit Order Book**: Price-time priority matching engine
- **Order Types**: Market and limit orders for BUY/SELL
- **Order Management**: Place, cancel, and track orders
- **Real-time Matching**: Instant order execution

### User Features
- **Authentication**: JWT-based user authentication
- **Portfolio Management**: Track stock ownership and balance
- **Order History**: View active and completed orders
- **Real-time Updates**: Live order book and position updates

### Technical Highlights
- **High Performance**: Optimized Go matching engine
- **Race Condition Safety**: Thread-safe order processing
- **PostgreSQL Integration**: Reliable data storage
- **REST API**: Clean backend API design
- **React Frontend**: Modern, responsive UI

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                   React Frontend                      │
│               (Order Management, Trading UI)           │
└───────────────────────┬───────────────────────────────┘
                        │ REST API (JSON)
┌───────────────────────▼───────────────────────────────┐
│                   Go Backend                          │
│               (Matching Engine, API Server)           │
└───────────────────────┬───────────────────────────────┘
                        │ SQL
┌───────────────────────▼───────────────────────────────┐
│                 PostgreSQL Database                   │
│               (Users, Orders, Transactions)          │
└───────────────────────────────────────────────────────┘
```

## Getting Started

### Prerequisites
- Go 1.20+
- Node.js 18+
- PostgreSQL 14+
- Docker (optional, for containerized deployment)

### Installation

1. **Clone the repository**:
```bash
git clone https://github.com/Andrebtk/Nexus---a-high-performance-matching-engine-in-GO.git
cd Nexus---a-high-performance-matching-engine-in-GO
```

2. **Set up PostgreSQL**:
```bash
createdb nexus
psql -d nexus -f database/schema.sql
```

3. **Configure environment**:
```bash
cp .env.example .env
# Edit .env with your PostgreSQL credentials
```

4. **Install dependencies**:
```bash
# Backend
go mod download

# Frontend
cd nexus-ui
npm install
```

### Running the Application

1. **Start the backend**:
```bash
go run cmd/api/main.go
# API will be available at http://localhost:8080
```

2. **Start the frontend**:
```bash
cd nexus-ui
npm run dev
# Frontend will be available at http://localhost:5173
```

## Project Structure

```
.
├── cmd/                # Main applications
│   └── api/            # API server
├── internal/           # Core application code
│   ├── api/            # API handlers
│   ├── database/       # Database models and migrations
│   ├── engine/         # Matching engine
│   ├── models/         # Data models
│   ├── services/       # Business logic
│   └── oracle/         # Market data oracle
├── nexus-ui/           # React frontend
│   ├── public/        # Static assets
│   ├── src/            # React source code
│   │   ├── components/ # React components
│   │   ├── context/    # React context
│   │   └── ...         # Other frontend code
├── go.mod              # Go module definition
├── go.sum              # Go dependencies
└── README.md           # This file
```

## Key Components

### Matching Engine (`internal/engine/`)

- **OrderBook**: Manages buy/sell orders at different price levels
- **Exchange**: Routes orders to appropriate order books
- **Limit**: Price level container with double-linked list for orders
- **OrderQueue**: Efficient order queue management

### API Layer (`internal/api/`)

- RESTful API endpoints for:
  - Authentication (login, register)
  - Order management (place, cancel, history)
  - Market data (order book, tickers)
  - User portfolio (balance, positions)

### Frontend (`nexus-ui/`)

- **Profile Page**: View orders, positions, and account balance
- **Trading Interface**: Place and manage orders
- **Order Book**: View market depth
- **Authentication**: Login/registration flow

## Development

### Backend Development

```bash
# Run tests
go test ./...

# Build backend
go build -o nexus cmd/api/main.go

# Run with hot reload (using air)
air
```

### Frontend Development

```bash
cd nexus-ui
npm run dev      # Development server
npm run build    # Production build
npm run lint     # Code linting
```

## Deployment

### Docker Deployment

```bash
# Build Docker image
docker build -t nexus-trading .

# Run container
docker run -p 8080:8080 \
  -e DB_HOST=your_postgres_host \
  -e DB_USER=your_postgres_user \
  -e DB_PASSWORD=your_postgres_password \
  -e DB_NAME=nexus \
  nexus-trading
```

### Production Recommendations

- Use environment variables for configuration
- Set up proper logging and monitoring
- Implement rate limiting
- Use HTTPS with proper certificates
- Set up database backups

## API Documentation

### Authentication
- `POST /auth/register` - User registration
- `POST /auth/login` - User login
- `GET /auth/me` - Get current user

### Orders
- `POST /order` - Place new order
- `POST /orders/:id/cancel` - Cancel order
- `GET /orders/active` - Get active orders
- `GET /orders/history` - Get order history

### Market Data
- `GET /tickers` - List available tickers
- `GET /book?symbol=SYMBOL` - Get order book
- `GET /current-prices` - Get current prices

### Portfolio
- `GET /auth/profile` - User profile
- `GET /auth/stock-ownership` - Stock ownership

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Create a new Pull Request

## License

This project is licensed under the MIT License.

## Contact

For questions or support, please open an issue on GitHub.
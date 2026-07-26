package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	// Get database connection string from environment variable
	// Default to local PostgreSQL if not set
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		dbConnStr = "postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable"
	}

	var err error
	DB, err = sql.Open("postgres", dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %v", err)
	}

	// Test the database connection
	err = DB.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	log.Println("Successfully connected to PostgreSQL database")

	// Create tables if they don't exist
	err = createTables()
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return DB, nil
}

func createTables() error {
	// Create users table
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) UNIQUE NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		balance DECIMAL(15, 2) DEFAULT 0.00,
		profit DECIMAL(15, 2) DEFAULT 0.00,
		loss DECIMAL(15, 2) DEFAULT 0.00,
		stock_ownership JSON DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := DB.Exec(usersTable)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Create orders table
	ordersTable := `
	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		symbol VARCHAR(10) NOT NULL,
		order_type VARCHAR(4) NOT NULL,
		quantity INTEGER NOT NULL,
		price DECIMAL(10, 2) NOT NULL,
		status VARCHAR(20) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(ordersTable)
	if err != nil {
		return fmt.Errorf("failed to create orders table: %v", err)
	}

	// Add stock_ownership column to users table if it doesn't exist
	_, err = DB.Exec(`
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS stock_ownership JSON DEFAULT '{}';
	`)
	if err != nil {
		return fmt.Errorf("failed to add stock_ownership column: %v", err)
	}

	// Create transactions table
	transactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		order_id VARCHAR(50),
		amount DECIMAL(15, 2) NOT NULL,
		type VARCHAR(20) NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(transactionsTable)
	if err != nil {
		return fmt.Errorf("failed to create transactions table: %v", err)
	}

	// Create cost_basis table
	costBasisTable := `
	CREATE TABLE IF NOT EXISTS cost_basis (
		user_id INTEGER NOT NULL,
		symbol VARCHAR(10) NOT NULL,
		quantity INTEGER NOT NULL DEFAULT 0,
		total_cost DECIMAL(15, 2) NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, symbol)
	);`

	_, err = DB.Exec(costBasisTable)
	if err != nil {
		return fmt.Errorf("failed to create cost_basis table: %v", err)
	}

	log.Println("Users table created/verified")
	log.Println("Orders table created/verified")
	log.Println("Stock ownership column added/verified")
	log.Println("Transactions table created/verified")
	log.Println("Cost basis table created/verified")

	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection closed")
	}
}
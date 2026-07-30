package services

import (
	"sync"
	"time"
	"Nexus/internal/models"
	"crypto/rand"
	"encoding/hex"
	"database/sql"
	"log"
)

type TransactionService struct {
	transactions map[string]*models.Transaction
	db           *sql.DB
	mu           sync.RWMutex
}

func NewTransactionService() *TransactionService {
	return &TransactionService{
		transactions: make(map[string]*models.Transaction),
	}
}

func NewPostgresTransactionService(db *sql.DB) *TransactionService {
	return &TransactionService{
		transactions: make(map[string]*models.Transaction),
		db:           db,
	}
}

func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err) // In production, handle this error more gracefully
	}
	return hex.EncodeToString(bytes)
}

func (ts *TransactionService) RecordTransaction(userID, orderID, transactionType string, amount float64) *models.Transaction {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	transaction := &models.Transaction{
		ID:        generateID(),
		UserID:    userID,
		OrderID:   orderID,
		Amount:    amount,
		Type:      transactionType,
		Timestamp: time.Now(),
	}

	ts.transactions[transaction.ID] = transaction

	// If we have a database connection, also persist to database
	if ts.db != nil {
		_, err := ts.db.Exec(`
			INSERT INTO transactions (user_id, order_id, amount, type, timestamp)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, orderID, amount, transactionType, transaction.Timestamp)
		if err != nil {
			// Don't log warnings for system_bot transactions (expected behavior)
			if userID != "system_bot" {
				log.Printf("Warning: Failed to persist transaction to database: %v", err)
			}
		}
	}

	return transaction
}

func (ts *TransactionService) GetTransactions(userID string) []*models.Transaction {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	// If we have a database connection, get transactions from database
	if ts.db != nil {
		return ts.getTransactionsFromDB(userID)
	}

	// Fallback to in-memory transactions
	var userTransactions []*models.Transaction

	for _, transaction := range ts.transactions {
		if transaction.UserID == userID {
			userTransactions = append(userTransactions, transaction)
		}
	}

	return userTransactions
}

func (ts *TransactionService) getTransactionsFromDB(userID string) []*models.Transaction {
	rows, err := ts.db.Query(`
		SELECT id, user_id, order_id, amount, type, timestamp
		FROM transactions
		WHERE user_id = $1
		ORDER BY timestamp DESC`, userID)
	if err != nil {
		log.Printf("Failed to get transactions from database: %v", err)
		return []*models.Transaction{}
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		var t models.Transaction
		var timestamp time.Time
		err := rows.Scan(&t.ID, &t.UserID, &t.OrderID, &t.Amount, &t.Type, &timestamp)
		if err != nil {
			log.Printf("Failed to scan transaction: %v", err)
			continue
		}
		t.Timestamp = timestamp
		transactions = append(transactions, &t)
	}

	return transactions
}

// GetAllTransactionsForUser gets all transactions for a user (including from database)
func (ts *TransactionService) GetAllTransactionsForUser(userID string) []*models.Transaction {
	if ts.db != nil {
		return ts.getTransactionsFromDB(userID)
	}
	return ts.GetTransactions(userID)
}
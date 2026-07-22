package services 

import (
	"sync"
	"time"
	"Nexus/internal/models"
	"crypto/rand"
	"encoding/hex"
)



type TransactionService struct {
	transactions map[string]*models.Transaction
	mu sync.RWMutex
}



func NewTransactionService() *TransactionService {
	return &TransactionService{
		transactions: make(map[string]*models.Transaction),
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
	return transaction
}


func (ts *TransactionService) GetTransactions(userID string) []*models.Transaction {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var userTransactions []*models.Transaction

	for _, transaction := range ts.transactions {
		if transaction.UserID == userID {
			userTransactions = append(userTransactions, transaction)
		}
	}

	return userTransactions
}
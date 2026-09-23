package services

import (
	"sync"
	"log"
	"strconv"
	"Nexus/internal/models"
)

type ProfitLossService struct {
	userService         interface{}
	postgresUserService *PostgresUserService
	transactionService *TransactionService
	mu                 sync.RWMutex
}

func NewProfitLossService(userService interface{}, postgresUserService *PostgresUserService, transactionService *TransactionService) *ProfitLossService {
	return &ProfitLossService{
		userService:         userService,
		postgresUserService: postgresUserService,
		transactionService: transactionService,
	}
}

func (pls *ProfitLossService) CalculateProfitLoss(userID string) (float64, float64, error) {
    pls.mu.Lock()
    defer pls.mu.Unlock()

    // For PostgreSQL users (numeric IDs), use database transactions
    // For in-memory users, use the in-memory transaction service
    var transactions []*models.Transaction

    // Check if this is a PostgreSQL user (numeric ID)
    if userID == "system_bot" {
        // This is an in-memory user, use in-memory transactions
        if userService, ok := pls.userService.(*UserService); ok {
            _, err := userService.GetUser(userID)
            if err != nil {
                return 0, 0, err
            }
        }
        transactions = pls.transactionService.GetTransactions(userID)
    } else {
        // This is a PostgreSQL user, use database transactions
        transactions = pls.transactionService.GetAllTransactionsForUser(userID)
    }

    var totalProfit, totalLoss float64

    for _, transaction := range transactions {
        if transaction.Type == "trade" {
            if transaction.Amount > 0 {
                totalProfit += transaction.Amount
            } else {
                totalLoss += transaction.Amount
            }
        }
    }

    // Update the user's profit/loss values
    // For PostgreSQL users, update the database
    // For in-memory users, update the in-memory service
    if userID == "system_bot" {
        // This is an in-memory user, update the in-memory service
        if userService, ok := pls.userService.(*UserService); ok {
            err := userService.UpdateProfitLoss(userID, totalProfit, totalLoss)
            if err != nil {
                log.Printf("ERROR: Failed to update profit/loss for user %s: %v", userID, err)
                return totalProfit, totalLoss, err
            }
        }
    } else {
        // This is a PostgreSQL user, update the database
        numericUserID, err := strconv.Atoi(userID)
        if err != nil {
            return totalProfit, totalLoss, err
        }
        if postgresUserService, ok := pls.userService.(*PostgresUserService); ok {
            err = postgresUserService.UpdateUserProfitLoss(numericUserID, totalProfit, totalLoss)
            if err != nil {
                log.Printf("ERROR: Failed to update profit/loss for PostgreSQL user %d: %v", numericUserID, err)
                return totalProfit, totalLoss, err
            }
        }
    }

    return totalProfit, totalLoss, nil
}

func (pls *ProfitLossService) GetUserProfitLoss(userID string) (float64, float64, error) {
	pls.mu.RLock()
	defer pls.mu.RUnlock()

	// Check if this is a PostgreSQL user (numeric ID)
	if userID == "system_bot" {
		// For in-memory users, get profit/loss from in-memory service
		if userService, ok := pls.userService.(*UserService); ok {
			user, err := userService.GetUser(userID)
			if err != nil {
				return 0, 0, err
			}
			return user.Profit, user.Loss, nil
		}
	} else {
		// For PostgreSQL users, get profit/loss from database
		numericUserID, err := strconv.Atoi(userID)
		if err != nil {
			return 0, 0, err
		}
		if pls.postgresUserService != nil {
			user, err := pls.postgresUserService.GetUserByID(numericUserID)
			if err != nil {
				return 0, 0, err
			}
			return user.Profit, user.Loss, nil
		}
	}

	return 0, 0, nil
}

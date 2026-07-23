package services 


import (
	"sync"
)


type ProfitLossService struct {
	userService         *UserService
	transactionService *TransactionService
	mu                 sync.RWMutex
}

func NewProfitLossService(userService *UserService, transactionService *TransactionService) *ProfitLossService {
	return &ProfitLossService{
		userService:         userService,
		transactionService: transactionService,
	}
}


func (pls *ProfitLossService) CalculateProfitLoss(userID string) (float64, float64, error) {
	pls.mu.Lock()
	defer pls.mu.Unlock()

	user, err := pls.userService.GetUser(userID)

	if err != nil {
		return 0, 0, err
	}

	transactions := pls.transactionService.GetTransactions(userID)
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

	user.Profit = totalProfit
	user.Loss = totalLoss
	return totalProfit, totalLoss, nil
}

func (pls *ProfitLossService) GetUserProfitLoss(userID string) (float64, float64, error) {
	pls.mu.RLock()
	defer pls.mu.RUnlock()

	user, err := pls.userService.GetUser(userID)
	if err != nil {
		return 0, 0, err
	}

	return user.Profit, user.Loss, nil
}

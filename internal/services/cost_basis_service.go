package services

import (
	"database/sql"
	"log"
)

type CostBasisService struct {
	db *sql.DB
}

func NewCostBasisService(db *sql.DB) *CostBasisService {
	return &CostBasisService{db: db}
}

// RecordBuy adds shares at the given price to the user's cost basis.
func (s *CostBasisService) RecordBuy(userID int, symbol string, quantity int, price float64) error {
	_, err := s.db.Exec(`
		INSERT INTO cost_basis (user_id, symbol, quantity, total_cost)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, symbol) DO UPDATE
		SET quantity = cost_basis.quantity + EXCLUDED.quantity,
		    total_cost = cost_basis.total_cost + EXCLUDED.total_cost`,
		userID, symbol, quantity, float64(quantity)*price,
	)
	if err != nil {
		log.Printf("ERROR: Failed to record buy for user %d, symbol %s: %v", userID, symbol, err)
	}
	return err
}

// RecordSell removes shares from the cost basis and returns the realized P&L.
func (s *CostBasisService) RecordSell(userID int, symbol string, quantity int, sellPrice float64) (float64, error) {
	var currentQty int
	var totalCost float64

	err := s.db.QueryRow(`
		SELECT quantity, total_cost FROM cost_basis
		WHERE user_id = $1 AND symbol = $2 FOR UPDATE`,
		userID, symbol,
	).Scan(&currentQty, &totalCost)

	if err == sql.ErrNoRows || currentQty == 0 {
		// No cost basis on record (legacy/edge case) — nothing to net against.
		log.Printf("WARNING: No cost basis found for user %d, symbol %s", userID, symbol)
		return 0, nil
	}
	if err != nil {
		log.Printf("ERROR: Failed to get cost basis for user %d, symbol %s: %v", userID, symbol, err)
		return 0, err
	}

	avgCost := totalCost / float64(currentQty)
	realized := (sellPrice - avgCost) * float64(quantity)

	newQty := currentQty - quantity
	newTotalCost := totalCost - avgCost*float64(quantity)
	if newQty <= 0 {
		newQty, newTotalCost = 0, 0
	}

	_, err = s.db.Exec(`
		UPDATE cost_basis SET quantity = $1, total_cost = $2
		WHERE user_id = $3 AND symbol = $4`,
		newQty, newTotalCost, userID, symbol,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update cost basis for user %d, symbol %s: %v", userID, symbol, err)
		return realized, err
	}

	log.Printf("INFO: Recorded sell for user %d, symbol %s: quantity=%d, sellPrice=$%.2f, avgCost=$%.2f, realized=$%.2f",
		userID, symbol, quantity, sellPrice, avgCost, realized)

	return realized, nil
}
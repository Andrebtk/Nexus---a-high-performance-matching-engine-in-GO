package services

import (
	"database/sql"
	"log"
	"time"
)

type OrderService struct {
	db *sql.DB
}

func NewOrderService(db *sql.DB) *OrderService {
	return &OrderService{db: db}
}
type Order struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	Symbol    string  `json:"symbol"`
	OrderType string  `json:"order_type"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func (s *OrderService) CreateOrder(userID int, symbol string, orderType string, quantity int, price float64, status string) (*Order, error) {
	query := `
	INSERT INTO orders (user_id, symbol, order_type, quantity, price, status)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at`

	row := s.db.QueryRow(query, userID, symbol, orderType, quantity, price, status)

	var order Order
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.Symbol,
		&order.OrderType,
		&order.Quantity,
		&order.Price,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		log.Printf("Failed to create order: %v", err)
		return nil, err
	}

	return &order, nil
}

func (s *OrderService) GetActiveOrders(userID int) ([]Order, error) {
	query := `
	SELECT id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at
	FROM orders
	WHERE user_id = $1 AND status = 'active'
	ORDER BY created_at DESC`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		log.Printf("Failed to get active orders: %v", err)
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.OrderType,
			&order.Quantity,
			&order.Price,
			&order.Status,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			log.Printf("Failed to scan order: %v", err)
			continue
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func (s *OrderService) GetOrderHistory(userID int) ([]Order, error) {
	query := `
	SELECT id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at
	FROM orders
	WHERE user_id = $1 AND status IN ('completed', 'cancelled')
	ORDER BY created_at DESC
	LIMIT 50`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		log.Printf("Failed to get order history: %v", err)
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Symbol,
			&order.OrderType,
			&order.Quantity,
			&order.Price,
			&order.Status,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			log.Printf("Failed to scan order: %v", err)
			continue
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func (s *OrderService) UpdateOrderStatus(orderID int, status string) error {
	query := `
	UPDATE orders
	SET status = $1, updated_at = CURRENT_TIMESTAMP
	WHERE id = $2`

	_, err := s.db.Exec(query, status, orderID)
	if err != nil {
		log.Printf("Failed to update order status: %v", err)
		return err
	}

	return nil
}

func (s *OrderService) CompleteOrder(orderID int) error {
	return s.UpdateOrderStatus(orderID, "completed")
}

func (s *OrderService) CancelOrder(orderID int) error {
	return s.UpdateOrderStatus(orderID, "cancelled")
}

// GetActiveSellQuantity returns the quantity currently reserved by the
// user's active (not yet completed/cancelled) sell orders for a symbol.
func (s *OrderService) GetActiveSellQuantity(userID int, symbol string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(quantity), 0)
		FROM orders
		WHERE user_id = $1 AND symbol = $2 AND order_type = 'SELL' AND status = 'active'`,
		userID, symbol,
	).Scan(&total)
	if err != nil {
		log.Printf("Failed to get active sell quantity: %v", err)
		return 0, err
	}
	return total, nil
}

// CompleteOrderByDetails updates order status to completed using order details instead of just ID
// This is used for orders that were created with DBOrderID=0 (system_bot orders)
func (s *OrderService) CompleteOrderByDetails(userID int, symbol string, price float64, timestamp time.Time) error {
	// First try to find the exact order by all details
	query := `
	UPDATE orders
	SET status = 'completed', updated_at = CURRENT_TIMESTAMP
	WHERE user_id = $1
	AND symbol = $2
	AND price = $3
	AND created_at = $4
	AND status = 'active'`

	result, err := s.db.Exec(query, userID, symbol, price, timestamp)
	if err != nil {
		log.Printf("Failed to complete order by details: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		// No order found with exact match, try with a small time window
		log.Printf("No order found with exact timestamp match, trying with time window")
		timeWindowQuery := `
		UPDATE orders
		SET status = 'completed', updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
		AND symbol = $2
		AND price = $3
		AND created_at BETWEEN $4 AND $5
		AND status = 'active'
		LIMIT 1`

		// Add 1 second window to handle rapid orders
		endTime := timestamp.Add(1 * time.Second)
		_, err = s.db.Exec(timeWindowQuery, userID, symbol, price, timestamp, endTime)
		if err != nil {
			log.Printf("Failed to complete order with time window: %v", err)
			return err
		}
	}

	return nil
}

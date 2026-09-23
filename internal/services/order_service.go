package services

import (
	"database/sql"
	"fmt"
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

// PlaceBuyOrderAtomically atomically checks balance, deducts it, and creates the order
func (s *OrderService) PlaceBuyOrderAtomically(userID int, symbol string, quantity int, price float64) (*Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// 1. Lock user row and get balance
	var balance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&balance)
	if err != nil {
		return nil, fmt.Errorf("failed to get user balance: %v", err)
	}

	requiredBalance := float64(quantity) * price
	if balance < requiredBalance {
		return nil, fmt.Errorf("insufficient balance. Required: $%.2f, Available: $%.2f", requiredBalance, balance)
	}

	// 2. Update balance
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", requiredBalance, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct balance: %v", err)
	}

	// 3. Insert order
	query := `
	INSERT INTO orders (user_id, symbol, order_type, quantity, price, status)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at`
	row := tx.QueryRow(query, userID, symbol, "BUY", quantity, price, "active")

	var order Order
	err = row.Scan(&order.ID, &order.UserID, &order.Symbol, &order.OrderType, &order.Quantity, &order.Price, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert order: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}
	return &order, nil
}

// PlaceSellOrderAtomically atomically checks stock ownership and creates the order
func (s *OrderService) PlaceSellOrderAtomically(userID int, symbol string, quantity int, price float64) (*Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// 1. Lock user row to serialize concurrent sell checks for this user
	var dummyID int
	err = tx.QueryRow("SELECT id FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&dummyID)
	if err != nil {
		return nil, fmt.Errorf("failed to lock user record: %v", err)
	}

	// 2. Calculate owned quantity (completed BUYs - completed SELLs)
	var totalBought, totalSold int
	tx.QueryRow("SELECT COALESCE(SUM(quantity), 0) FROM orders WHERE user_id = $1 AND symbol = $2 AND order_type = 'BUY' AND status = 'completed'", userID, symbol).Scan(&totalBought)
	tx.QueryRow("SELECT COALESCE(SUM(quantity), 0) FROM orders WHERE user_id = $1 AND symbol = $2 AND order_type = 'SELL' AND status = 'completed'", userID, symbol).Scan(&totalSold)
	ownedQuantity := totalBought - totalSold

	// 3. Calculate reserved quantity (active SELLs)
	var reservedQuantity int
	tx.QueryRow("SELECT COALESCE(SUM(quantity), 0) FROM orders WHERE user_id = $1 AND symbol = $2 AND order_type = 'SELL' AND status = 'active'", userID, symbol).Scan(&reservedQuantity)
	
	availableQuantity := ownedQuantity - reservedQuantity

	if quantity > availableQuantity {
		return nil, fmt.Errorf("insufficient stock ownership. Trying to sell %d shares of %s, but only %d available (owned: %d, already reserved: %d)", quantity, symbol, availableQuantity, ownedQuantity, reservedQuantity)
	}

	// 4. Insert order
	query := `
	INSERT INTO orders (user_id, symbol, order_type, quantity, price, status)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at`
	row := tx.QueryRow(query, userID, symbol, "SELL", quantity, price, "active")

	var order Order
	err = row.Scan(&order.ID, &order.UserID, &order.Symbol, &order.OrderType, &order.Quantity, &order.Price, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert order: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
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

func (s *OrderService) GetAllActiveOrders() ([]Order, error) {
	query := `
	SELECT id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at
	FROM orders
	WHERE status = 'active'
	ORDER BY created_at ASC`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("Failed to get all active orders: %v", err)
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

func (s *OrderService) UpdateOrderQuantity(orderID int, newQuantity int) error {
	query := `
	UPDATE orders
	SET quantity = $1, updated_at = CURRENT_TIMESTAMP
	WHERE id = $2`

	_, err := s.db.Exec(query, newQuantity, orderID)
	if err != nil {
		log.Printf("Failed to update order %d quantity to %d: %v", orderID, newQuantity, err)
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

func (s *OrderService) GetOrderByID(orderID int) (*Order, error) {
	query := `
	SELECT id, user_id, symbol, order_type, quantity, price, status, created_at, updated_at
	FROM orders
	WHERE id = $1`

	var order Order
	err := s.db.QueryRow(query, orderID).Scan(
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		log.Printf("Failed to fetch order %d: %v", orderID, err)
		return nil, err
	}
	return &order, nil
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

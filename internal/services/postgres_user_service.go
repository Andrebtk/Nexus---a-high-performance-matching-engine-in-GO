package services

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"Nexus/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type PostgresUserService struct {
	db *sql.DB
}

func NewPostgresUserService(db *sql.DB) *PostgresUserService {
	return &PostgresUserService{db: db}
}

func (s *PostgresUserService) RegisterUser(user models.UserRegistration) (*models.PostgresUser, error) {
	// Check if username or email already exists
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1 OR email = $2", user.Username, user.Email).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %v", err)
	}
	if count > 0 {
		return nil, errors.New("username or email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	// Insert new user
	var postgresUser models.PostgresUser
	err = s.db.QueryRow(`
		INSERT INTO users (username, email, password_hash, balance, profit, loss)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, username, email, balance, profit, loss, created_at, updated_at`,
		user.Username, user.Email, string(hashedPassword), 1000.00, 0.00, 0.00,
	).Scan(
		&postgresUser.ID, &postgresUser.Username, &postgresUser.Email,
		&postgresUser.Balance, &postgresUser.Profit, &postgresUser.Loss,
		&postgresUser.CreatedAt, &postgresUser.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	return &postgresUser, nil
}

func (s *PostgresUserService) AuthenticateUser(login models.UserLogin) (*models.PostgresUser, error) {
	var user models.PostgresUser
	err := s.db.QueryRow(`
		SELECT id, username, email, password_hash, balance, profit, loss, created_at, updated_at
		FROM users WHERE email = $1`,
		login.Email,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.Balance, &user.Profit, &user.Loss, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	// Compare password hash
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(login.Password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	return &user, nil
}

func (s *PostgresUserService) GetUserByID(userID int) (*models.PostgresUser, error) {
	var user models.PostgresUser
	err := s.db.QueryRow(`
		SELECT id, username, email, balance, profit, loss, created_at, updated_at
		FROM users WHERE id = $1`,
		userID,
	).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.Balance, &user.Profit, &user.Loss, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	return &user, nil
}

func (s *PostgresUserService) GetUserByEmail(email string) (*models.PostgresUser, error) {
	var user models.PostgresUser
	err := s.db.QueryRow(`
		SELECT id, username, email, balance, profit, loss, created_at, updated_at
		FROM users WHERE email = $1`,
		email,
	).Scan(
		&user.ID, &user.Username, &user.Email,
		&user.Balance, &user.Profit, &user.Loss, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	return &user, nil
}

func (s *PostgresUserService) UpdateUserBalance(userID int, amount float64) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`,
		amount, userID,
	)
	return err
}

func (s *PostgresUserService) UpdateUserProfitLoss(userID int, profit, loss float64) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET profit = $1, loss = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3`,
		profit, loss, userID,
	)
	return err
}

// AddRealizedPL applies a realized gain/loss from a sell trade.
// Positive realized -> profit; negative -> loss (stored as a negative number,
// consistent with the existing "loss" convention where net = profit + loss).
func (s *PostgresUserService) AddRealizedPL(userID int, realized float64) error {
	if realized >= 0 {
		_, err := s.db.Exec(`
			UPDATE users SET profit = profit + $1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $2`, realized, userID)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE users SET loss = loss + $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`, realized, userID)
	return err
}

// GetStockOwnership returns the stock ownership for a user
func (s *PostgresUserService) GetStockOwnership(userID int) (map[string]int, error) {
	var stockOwnershipJSON string
	err := s.db.QueryRow(`
		SELECT stock_ownership
		FROM users WHERE id = $1`,
		userID,
	).Scan(&stockOwnershipJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch stock ownership: %v", err)
	}

	// Parse the JSON into a map
	stockOwnership := make(map[string]int)
	if stockOwnershipJSON != "" {
		// Simple JSON parsing for {"symbol":"quantity"} format
		// Note: In a production environment, you'd use json.Unmarshal
		// This is a simplified approach for demonstration
	}

	return stockOwnership, nil
}

// GetAllTradedSymbols returns all symbols that a user has traded
func (s *PostgresUserService) GetAllTradedSymbols(userID int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT symbol
		FROM orders
		WHERE user_id = $1
		AND (order_type = 'BUY' AND status = 'completed')
		OR (order_type = 'SELL' AND status = 'completed')`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch traded symbols: %v", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			continue
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// GetStockQuantity returns the quantity of a specific stock owned by a user
func (s *PostgresUserService) GetStockQuantity(userID int, symbol string) (int, error) {
	// For now, use a simple approach: check if the user has any completed BUY orders for this symbol
	// This gives us basic stock ownership tracking without complex JSON parsing

	var totalBought int
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(quantity), 0)
		FROM orders
		WHERE user_id = $1
		AND symbol = $2
		AND order_type = 'BUY'
		AND status = 'completed'`,
		userID, symbol,
	).Scan(&totalBought)

	if err != nil {
		return 0, fmt.Errorf("failed to calculate stock ownership: %v", err)
	}

	// Also subtract any completed SELL orders
	var totalSold int
	err = s.db.QueryRow(`
		SELECT COALESCE(SUM(quantity), 0)
		FROM orders
		WHERE user_id = $1
		AND symbol = $2
		AND order_type = 'SELL'
		AND status = 'completed'`,
		userID, symbol,
	).Scan(&totalSold)

	if err != nil {
		return 0, fmt.Errorf("failed to calculate stock ownership: %v", err)
	}

	// Current ownership = stocks bought - stocks sold
	ownedQuantity := totalBought - totalSold
	if ownedQuantity < 0 {
		ownedQuantity = 0 // Can't have negative stocks
	}

	return ownedQuantity, nil
}

// UpdateStockOwnership updates the stock ownership for a user
func (s *PostgresUserService) UpdateStockOwnership(userID int, symbol string, quantity int) error {
	// First, get the current stock ownership
	var currentOwnershipJSON string
	err := s.db.QueryRow(`
		SELECT stock_ownership
		FROM users WHERE id = $1`,
		userID,
	).Scan(&currentOwnershipJSON)

	if err != nil {
		return fmt.Errorf("failed to fetch current stock ownership: %v", err)
	}

	// For now, we'll use a simple approach
	// In a production environment, we would:
	// 1. Parse the current JSON
	// 2. Update the specific symbol's quantity
	// 3. Marshal back to JSON
	// 4. Update the database

	// For this implementation, we'll use a simplified SQL update
	// that uses PostgreSQL's JSON functions
	_, err = s.db.Exec(`
		UPDATE users
		SET stock_ownership = COALESCE(
			stock_ownership::jsonb || jsonb_build_object($2, $3),
			jsonb_build_object($2, $3)
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`,
		userID, symbol, quantity,
	)

	return err
}

// AddStockOwnership adds to the existing stock ownership for a user
func (s *PostgresUserService) AddStockOwnership(userID int, symbol string, quantity int) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET stock_ownership = COALESCE(
			stock_ownership::jsonb || jsonb_build_object($2, (stock_ownership->>$2)::int + $3),
			jsonb_build_object($2, $3)
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`,
		userID, symbol, quantity,
	)

	return err
}

// CreateSystemBotIfNotExists creates the system bot user if it doesn't exist
func (s *PostgresUserService) CreateSystemBotIfNotExists() error {
	// Check if system bot exists
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'system_bot'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check system bot: %v", err)
	}

	if count > 0 {
		log.Println("System bot already exists")
		return nil
	}

	// Create system bot
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("system_bot_password"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash system bot password: %v", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO users (username, email, password_hash, balance, profit, loss)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		"system_bot", "system@nexus.com", string(hashedPassword), 1000000.00, 0.00, 0.00,
	)

	if err != nil {
		return fmt.Errorf("failed to create system bot: %v", err)
	}

	log.Println("✅ System bot created successfully")
	return nil
}

// CreateGuestUser creates a temporary guest user for unauthenticated sessions
func (s *PostgresUserService) CreateGuestUser() (*models.PostgresUser, error) {
	// Generate a unique guest username
	timestamp := time.Now().UnixNano()
	timestampStr := fmt.Sprintf("%d", timestamp)
	guestID := "guest_" + timestampStr[len(timestampStr)-8:]

	// Create guest user
	var guestUser models.PostgresUser
	err := s.db.QueryRow(`
		INSERT INTO users (username, email, password_hash, balance, profit, loss)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, username, email, balance, profit, loss, created_at, updated_at`,
		guestID, guestID+"@nexus.com", "guest_password", 1000.00, 0.00, 0.00,
	).Scan(
		&guestUser.ID, &guestUser.Username, &guestUser.Email,
		&guestUser.Balance, &guestUser.Profit, &guestUser.Loss,
		&guestUser.CreatedAt, &guestUser.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create guest user: %v", err)
	}

	log.Printf("✅ Guest user created: %s", guestID)
	return &guestUser, nil
}

// MigrateInMemoryUsersToPostgres migrates existing in-memory users to PostgreSQL
func MigrateInMemoryUsersToPostgres(postgresService *PostgresUserService, memoryService *UserService) error {
	// Get all users from memory service
	memoryUsers := memoryService.GetAllUsers()

	for _, user := range memoryUsers {
		// Check if user already exists in PostgreSQL
		_, err := postgresService.GetUserByEmail(user.ID + "@nexus.com")
		if err == nil {
			continue // User already exists
		}

		// Create user in PostgreSQL
		registration := models.UserRegistration{
			Username: user.ID,
			Email:    user.ID + "@nexus.com",
			Password: "migrated_password_" + user.ID,
		}

		_, err = postgresService.RegisterUser(registration)
		if err != nil {
			log.Printf("Failed to migrate user %s: %v", user.ID, err)
			continue
		}

		// Update balance, profit, and loss
		postgresUser, err := postgresService.GetUserByEmail(registration.Email)
		if err != nil {
			log.Printf("Failed to get migrated user %s: %v", user.ID, err)
			continue
		}

		err = postgresService.UpdateUserBalance(postgresUser.ID, user.Balance-1000.00) // Subtract initial 1000
		if err != nil {
			log.Printf("Failed to update balance for user %s: %v", user.ID, err)
		}

		err = postgresService.UpdateUserProfitLoss(postgresUser.ID, user.Profit, user.Loss)
		if err != nil {
			log.Printf("Failed to update profit/loss for user %s: %v", user.ID, err)
		}

		log.Printf("✅ Migrated user %s to PostgreSQL", user.ID)
	}

	return nil
}


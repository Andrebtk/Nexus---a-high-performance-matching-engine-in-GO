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


package services


import (
	"errors"
	"sync"
	"Nexus/internal/models"
)



type UserService struct {
	users map[string]*models.User
	mu sync.RWMutex
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[string]*models.User),
	}
}


func (us *UserService) CreateUser(id string) (*models.User, error) {
	us.mu.Lock()
	defer us.mu.Unlock()

	if _, exists := us.users[id]; exists {
		return nil, errors.New("user already exists")
	}

	user := &models.User{
		ID:      id,
		Balance: 0.0,
		Profit:  0.0,
		Loss:    0.0,
	}

	us.users[id] = user
	return user, nil
}


func (us *UserService) GetUser(id string) (*models.User, error) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	user, exists := us.users[id]
	
	if !exists {
		return nil, errors.New("user not found")
	}

	return user, nil
}


func (us *UserService) Deposit(id string, amount float64) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	user, exists := us.users[id]
	if !exists {
		return errors.New("user not found")
	}

	user.Balance += amount
	return nil
}


func (us *UserService) Withdraw(id string, amount float64) error {
	us.mu.Lock()
	defer us.mu.Unlock()

	user, exists := us.users[id]
	if !exists {
		return errors.New("user not found")
	}

	if user.Balance < amount {
		return errors.New("insufficient balance")
	}

	user.Balance -= amount
	return nil
}
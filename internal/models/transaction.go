package models

import "time"

type Transaction struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    OrderID   string    `json:"order_id"`
    Amount    float64   `json:"amount"`
    Type      string    `json:"type"` // "deposit", "withdrawal", "trade"
    Timestamp time.Time `json:"timestamp"`
}
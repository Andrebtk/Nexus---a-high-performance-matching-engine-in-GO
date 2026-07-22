package models

type User struct {
	ID      string  `json:"id"`
	Balance float64 `json:"balance"`
	Profit  float64 `json:"profit"`
	Loss    float64 `json:"loss"`
}



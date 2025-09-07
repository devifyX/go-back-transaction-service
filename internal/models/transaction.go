package models


import "time"


type Transaction struct {
	ID string `json:"id"`
	CoinID string `json:"coinid"`
	UserID string `json:"userid"`
	DataID string `json:"dataid"`
	CoinUsed float64 `json:"coinused"`
	TransactionTimestamp time.Time `json:"transactionTimestamp"`
	ExpiryDate time.Time `json:"expiryDate"`
	PlatformName string `json:"platformName"`
}
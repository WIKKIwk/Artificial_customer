package entity

import "time"

// Message domain entity
type Message struct {
	ID        string
	UserID    int64
	Username  string
	Text      string
	Response  string
	Timestamp time.Time
}

// ChatContext suhbat kontekstini saqlash uchun
type ChatContext struct {
	UserID   int64
	Messages []Message
	LastUsed time.Time
}

package entity

import "time"

// Product mahsulot entity
type Product struct {
	ID          string
	Name        string
	Category    string
	Price       float64
	Description string
	Stock       int
	Specs       map[string]string // Texnik xususiyatlar
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProductCatalog mahsulotlar katalogi
type ProductCatalog struct {
	Products  []Product
	UpdatedAt time.Time
	Source    string // Excel fayl nomi
}

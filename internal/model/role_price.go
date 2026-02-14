package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RolePrice stores price for each user role (user_type) - untuk payment upgrade role
type RolePrice struct {
	ID          string    `gorm:"type:uuid;primary_key" json:"id"`
	Role        string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"role"` // member, premium, vip, etc (sama dengan user_type)
	Name        string    `gorm:"type:varchar(255)" json:"name"`                      // Nama tampilan: "Premium", "VIP"
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Price       int       `gorm:"not null" json:"price"` // Harga dalam satuan terkecil (rupiah)
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name
func (RolePrice) TableName() string {
	return "role_prices"
}

// BeforeCreate hook to generate UUID
func (r *RolePrice) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

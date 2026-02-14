package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentStatus represents payment status
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSuccess   PaymentStatus = "success"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCancelled PaymentStatus = "cancelled"
	PaymentStatusExpired   PaymentStatus = "expired"
)

// Payment represents a payment record (Midtrans)
type Payment struct {
	ID                     string         `gorm:"type:uuid;primary_key" json:"id"`
	UserID                 string         `gorm:"type:uuid;not null;index" json:"user_id"`
	OrderID                string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"order_id"`
	Amount                 int            `gorm:"not null" json:"amount"`
	TotalAmount            int            `gorm:"not null" json:"total_amount"`
	Status                 PaymentStatus  `gorm:"type:varchar(20);default:'pending'" json:"status"`
	PaymentMethod          string         `gorm:"type:varchar(50)" json:"payment_method"`
	PaymentType            string         `gorm:"type:varchar(50);default:'midtrans'" json:"payment_type"`
	ItemName               string         `gorm:"type:varchar(255)" json:"item_name"`
	ItemCategory           string         `gorm:"type:varchar(100);default:'digital'" json:"item_category"`
	Description            string         `gorm:"type:text" json:"description,omitempty"`
	CustomerName           string         `gorm:"type:varchar(255)" json:"customer_name"`
	CustomerEmail          string         `gorm:"type:varchar(255)" json:"customer_email"`
	MidtransTransactionID  string         `gorm:"type:varchar(100);index" json:"midtrans_transaction_id,omitempty"`
	FraudStatus            string         `gorm:"type:varchar(50)" json:"fraud_status,omitempty"`
	VANumber               string         `gorm:"type:varchar(50)" json:"va_number,omitempty"`
	BankType               string         `gorm:"type:varchar(50)" json:"bank_type,omitempty"`
	QRCodeURL              string         `gorm:"type:text" json:"qr_code_url,omitempty"`
	RedirectURL            string         `gorm:"type:text" json:"redirect_url,omitempty"`
	MaskedCard             string         `gorm:"type:varchar(50)" json:"masked_card,omitempty"`
	CardType               string         `gorm:"type:varchar(50)" json:"card_type,omitempty"`
	SavedTokenID           string         `gorm:"type:varchar(255)" json:"saved_token_id,omitempty"`
	ExpiryTime             *time.Time     `json:"expiry_time,omitempty"`
	MidtransResponse       string         `gorm:"type:text" json:"-"`
	Metadata               string         `gorm:"type:text" json:"metadata,omitempty"` // JSON: { "target_type": "post", "target_id": "uuid" }
	TargetRole             string         `gorm:"type:varchar(50);index" json:"target_role,omitempty"` // Role yang dibeli (upgrade user_type)
	CreatedAt              time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name
func (Payment) TableName() string {
	return "payments"
}

// BeforeCreate hook to generate UUID
func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

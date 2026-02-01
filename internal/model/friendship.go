package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Friendship struct {
	ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SenderID   string    `gorm:"type:uuid;not null;index" json:"sender_id"`
	ReceiverID string    `gorm:"type:uuid;not null;index" json:"receiver_id"`
	Status     string    `gorm:"type:varchar(20);default:'pending';not null" json:"status"` // pending, accepted, rejected, blocked
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationships
	Sender   User `gorm:"foreignKey:SenderID;references:ID" json:"sender,omitempty"`
	Receiver User `gorm:"foreignKey:ReceiverID;references:ID" json:"receiver,omitempty"`
}

// BeforeCreate hook to generate UUID
func (f *Friendship) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Friendship) TableName() string {
	return "friendships"
}

// Friendship status constants
const (
	FriendshipStatusPending  = "pending"
	FriendshipStatusAccepted = "accepted"
	FriendshipStatusRejected = "rejected"
	FriendshipStatusBlocked  = "blocked"
)

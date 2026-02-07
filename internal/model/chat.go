package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatMessage represents a direct message between two users
type ChatMessage struct {
	ID        string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SenderID  string         `gorm:"type:uuid;not null;index" json:"sender_id"`
	ReceiverID string        `gorm:"type:uuid;not null;index" json:"receiver_id"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	IsRead    bool           `gorm:"default:false" json:"is_read"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Sender   User `gorm:"foreignKey:SenderID;references:ID" json:"sender,omitempty"`
	Receiver User `gorm:"foreignKey:ReceiverID;references:ID" json:"receiver,omitempty"`
}

// BeforeCreate hook
func (c *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (ChatMessage) TableName() string {
	return "chat_messages"
}

package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Notification struct {
	ID        string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    string     `gorm:"type:uuid;not null;index" json:"user_id"`
	SenderID  *string    `gorm:"type:uuid;index" json:"sender_id,omitempty"` // Optional: who sent the notification (e.g., friend request sender)
	Type      string     `gorm:"type:varchar(50);not null" json:"type"`      // friend_request, friend_accepted, friend_rejected, etc.
	Title     string     `gorm:"type:varchar(255);not null" json:"title"`
	Message   string     `gorm:"type:text" json:"message"`
	TargetID  *string    `gorm:"type:uuid;index" json:"target_id,omitempty"` // Optional: ID of the related entity (e.g., friendship ID, post ID)
	Data      string     `gorm:"type:jsonb" json:"data,omitempty"`           // Additional data in JSON format
	IsRead    bool       `gorm:"default:false" json:"is_read"`
	ReadAt    *time.Time `gorm:"type:timestamp" json:"read_at,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationships
	User   User  `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	Sender *User `gorm:"foreignKey:SenderID;references:ID" json:"sender,omitempty"`
}

// BeforeCreate hook to generate UUID
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Notification) TableName() string {
	return "notifications"
}

// Notification type constants
const (
	NotificationTypeFriendRequest       = "friend_request"
	NotificationTypeFriendAccepted      = "friend_accepted"
	NotificationTypeFriendRejected      = "friend_rejected"
	NotificationTypeFriendRemoved       = "friend_removed"
	NotificationTypeCommentReply        = "comment_reply"
	NotificationTypePostComment         = "post_comment"
	NotificationTypePostUploadCompleted = "post_upload_completed"
	NotificationTypePostLiked           = "post_liked"
	NotificationTypeRoleUpdated         = "role_updated"
	NotificationTypeRolePurchased       = "role_purchased"
)

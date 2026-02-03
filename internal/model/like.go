package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Like struct {
	ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID     string    `gorm:"type:uuid;not null;index:idx_user_target,unique" json:"user_id"`
	TargetType string    `gorm:"type:varchar(20);not null;index:idx_user_target,unique" json:"target_type"` // post, comment
	TargetID   string    `gorm:"type:uuid;not null;index:idx_user_target,unique" json:"target_id"`
	Reaction   string    `gorm:"type:varchar(20);default:'like'" json:"reaction"` // like, love, haha, wow, sad, angry
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (l *Like) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Like) TableName() string {
	return "likes"
}

// Constants for target types
const (
	TargetTypePost    = "post"
	TargetTypeComment = "comment"
)

// Constants for reactions
const (
	ReactionLike  = "like"
	ReactionLove  = "love"
	ReactionHaha  = "haha"
	ReactionWow   = "wow"
	ReactionSad   = "sad"
	ReactionAngry = "angry"
)

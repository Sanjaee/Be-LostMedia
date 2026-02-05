package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostView struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PostID    string    `gorm:"type:uuid;not null;index:idx_post_user,unique" json:"post_id"`
	UserID    string    `gorm:"type:uuid;not null;index:idx_post_user,unique" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Post Post `gorm:"foreignKey:PostID;references:ID" json:"post,omitempty"`
	User User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (pv *PostView) BeforeCreate(tx *gorm.DB) error {
	if pv.ID == "" {
		pv.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (PostView) TableName() string {
	return "post_views"
}

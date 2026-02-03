package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Comment struct {
	ID        string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PostID    string         `gorm:"type:uuid;not null;index;references:posts(id)" json:"post_id"`
	UserID    string         `gorm:"type:uuid;not null;index;references:users(id)" json:"user_id"`
	ParentID  *string        `gorm:"type:uuid;index;references:comments(id)" json:"parent_id,omitempty"` // For nested comments/replies
	Content   string         `gorm:"type:text;not null" json:"content"`
	MediaURL  *string        `gorm:"type:text" json:"media_url,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Post    Post      `gorm:"foreignKey:PostID;references:ID" json:"post,omitempty"`
	User    User      `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	Parent  *Comment  `gorm:"foreignKey:ParentID;references:ID" json:"parent,omitempty"`
	Replies []Comment `gorm:"foreignKey:ParentID;references:ID" json:"replies,omitempty"`
	// Likes relationship is polymorphic (target_type + target_id), so we don't use foreign key constraint
	// Likes are accessed via service layer using TargetID and TargetType
	LikeCount int64 `gorm:"-" json:"like_count"` // Virtual field, calculated
}

// BeforeCreate hook to generate UUID
func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Comment) TableName() string {
	return "comments"
}

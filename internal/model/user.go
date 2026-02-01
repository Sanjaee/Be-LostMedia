package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID             string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email          string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Username       *string        `gorm:"type:varchar(100);uniqueIndex" json:"username,omitempty"`
	Phone          *string        `gorm:"type:varchar(20)" json:"phone,omitempty"`
	FullName       string         `gorm:"type:varchar(255);not null" json:"full_name"`
	PasswordHash   string         `gorm:"type:varchar(255)" json:"-"`
	UserType       string         `gorm:"type:varchar(50);default:'member'" json:"user_type"`
	ProfilePhoto   *string        `gorm:"type:text" json:"profile_photo,omitempty"`
	DateOfBirth    *time.Time     `gorm:"type:date" json:"date_of_birth,omitempty"`
	Gender         *string        `gorm:"type:varchar(20)" json:"gender,omitempty"`
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	IsVerified     bool           `gorm:"default:false" json:"is_verified"`
	LastLogin      *time.Time     `gorm:"type:timestamp" json:"last_login,omitempty"`
	LoginType      string         `gorm:"type:varchar(50);default:'credential'" json:"login_type"` // credential, google
	GoogleID       *string        `gorm:"type:varchar(255);uniqueIndex" json:"-"`
	OTPCode        *string        `gorm:"type:varchar(6)" json:"-"`
	OTPExpiresAt   *time.Time     `gorm:"type:timestamp" json:"-"`
	ResetToken     *string        `gorm:"type:text" json:"-"`
	ResetExpiresAt *time.Time     `gorm:"type:timestamp" json:"-"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate hook to generate UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Profile struct {
	ID                 string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID             string    `gorm:"type:uuid;not null;uniqueIndex;references:users(id)" json:"user_id"`
	Bio                *string   `gorm:"type:text" json:"bio,omitempty"`
	CoverPhoto         *string   `gorm:"type:text" json:"cover_photo,omitempty"`
	Website            *string   `gorm:"type:varchar(500)" json:"website,omitempty"`
	Location           *string   `gorm:"type:varchar(255)" json:"location,omitempty"`
	City               *string   `gorm:"type:varchar(255)" json:"city,omitempty"`
	Country            *string   `gorm:"type:varchar(255)" json:"country,omitempty"`
	Hometown           *string   `gorm:"type:varchar(255)" json:"hometown,omitempty"`
	Education          *string   `gorm:"type:varchar(500)" json:"education,omitempty"`
	Work               *string   `gorm:"type:varchar(500)" json:"work,omitempty"`
	RelationshipStatus *string   `gorm:"type:varchar(50)" json:"relationship_status,omitempty"`
	Intro              *string   `gorm:"type:text" json:"intro,omitempty"`
	IsProfilePublic    bool      `gorm:"default:true" json:"is_profile_public"`
	CreatedAt          time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationship
	User User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (p *Profile) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Profile) TableName() string {
	return "profiles"
}

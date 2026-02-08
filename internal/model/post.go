package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Post struct {
	ID           string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       string         `gorm:"type:uuid;not null;index;references:users(id)" json:"user_id"`
	GroupID      *string        `gorm:"type:uuid;index;references:groups(id)" json:"group_id,omitempty"`
	Content      *string        `gorm:"type:text" json:"content,omitempty"`
	ImageURLs    string         `gorm:"type:jsonb" json:"image_urls,omitempty"` // Array of image URLs stored as JSON
	SharedPostID *string        `gorm:"type:uuid;index;references:posts(id)" json:"shared_post_id,omitempty"`
	IsPinned     bool           `gorm:"default:false" json:"is_pinned"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed fields for API response (not in DB)
	LikesCount    int64 `json:"likes_count,omitempty" gorm:"-"`
	CommentsCount int64 `json:"comments_count,omitempty" gorm:"-"`
	UserLiked     bool  `json:"user_liked,omitempty" gorm:"-"`

	// Relationships
	User       User          `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	Group      *Group        `gorm:"foreignKey:GroupID;references:ID" json:"group,omitempty"`
	SharedPost *Post         `gorm:"foreignKey:SharedPostID;references:ID" json:"shared_post,omitempty"`
	Tags       []PostTag     `gorm:"foreignKey:PostID;references:ID" json:"tags,omitempty"`
	Location   *PostLocation `gorm:"foreignKey:PostID;references:ID" json:"location,omitempty"`
}

// BeforeCreate hook to generate UUID
func (p *Post) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Post) TableName() string {
	return "posts"
}

// GetImageURLs returns ImageURLs as a slice of strings
func (p *Post) GetImageURLs() []string {
	if p.ImageURLs == "" || p.ImageURLs == "[]" {
		return []string{}
	}
	var urls []string
	if err := json.Unmarshal([]byte(p.ImageURLs), &urls); err != nil {
		return []string{}
	}
	return urls
}

// SetImageURLs sets ImageURLs from a slice of strings
func (p *Post) SetImageURLs(urls []string) error {
	if len(urls) == 0 {
		// Use empty JSON array instead of empty string for PostgreSQL JSONB
		p.ImageURLs = "[]"
		return nil
	}
	bytes, err := json.Marshal(urls)
	if err != nil {
		return err
	}
	p.ImageURLs = string(bytes)
	return nil
}

// MarshalJSON custom JSON marshaling to convert ImageURLs string to array
func (p *Post) MarshalJSON() ([]byte, error) {
	type Alias Post
	aux := &struct {
		ImageURLs []string `json:"image_urls,omitempty"`
		*Alias
	}{
		ImageURLs: p.GetImageURLs(),
		Alias:     (*Alias)(p),
	}
	return json.Marshal(aux)
}

// PostTag represents a tagged user in a post
type PostTag struct {
	ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PostID       string    `gorm:"type:uuid;not null;index:idx_post_tag,unique;references:posts(id)" json:"post_id"`
	TaggedUserID string    `gorm:"type:uuid;not null;index:idx_post_tag,unique;references:users(id)" json:"tagged_user_id"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Post       Post `gorm:"foreignKey:PostID;references:ID" json:"post,omitempty"`
	TaggedUser User `gorm:"foreignKey:TaggedUserID;references:ID" json:"tagged_user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (pt *PostTag) BeforeCreate(tx *gorm.DB) error {
	if pt.ID == "" {
		pt.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (PostTag) TableName() string {
	return "post_tags"
}

// PostLocation represents a location in a post
type PostLocation struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PostID    string    `gorm:"type:uuid;not null;uniqueIndex;references:posts(id)" json:"post_id"`
	PlaceName *string   `gorm:"type:varchar(255)" json:"place_name,omitempty"`
	Latitude  *float64  `gorm:"type:float" json:"latitude,omitempty"`
	Longitude *float64  `gorm:"type:float" json:"longitude,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Post Post `gorm:"foreignKey:PostID;references:ID" json:"post,omitempty"`
}

// BeforeCreate hook to generate UUID
func (pl *PostLocation) BeforeCreate(tx *gorm.DB) error {
	if pl.ID == "" {
		pl.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (PostLocation) TableName() string {
	return "post_locations"
}

// Group model
type Group struct {
	ID               string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedBy        string         `gorm:"type:uuid;not null;references:users(id)" json:"created_by"`
	Name             string         `gorm:"type:varchar(255);not null" json:"name"`
	Slug             string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"slug"`
	Description      *string        `gorm:"type:text" json:"description,omitempty"`
	CoverPhoto       *string        `gorm:"type:text" json:"cover_photo,omitempty"`
	Icon             *string        `gorm:"type:text" json:"icon,omitempty"`
	Privacy          string         `gorm:"type:varchar(20);default:'open'" json:"privacy"`               // open, closed, secret
	MembershipPolicy string         `gorm:"type:varchar(30);default:'anyone_can_join'" json:"membership_policy"` // anyone_can_join, approval_required
	IsActive         bool           `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Creator User          `gorm:"foreignKey:CreatedBy;references:ID" json:"creator,omitempty"`
	Members []GroupMember `gorm:"foreignKey:GroupID;references:ID" json:"members,omitempty"`

	// Computed
	MembersCount int64 `json:"members_count,omitempty" gorm:"-"`
}

// BeforeCreate hook to generate UUID
func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (Group) TableName() string {
	return "groups"
}

// GroupMember represents a user's membership in a group
type GroupMember struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GroupID   string    `gorm:"type:uuid;not null;index:idx_group_member,unique" json:"group_id"`
	UserID    string    `gorm:"type:uuid;not null;index:idx_group_member,unique" json:"user_id"`
	Role      string    `gorm:"type:varchar(20);default:'member'" json:"role"` // admin, moderator, member
	Status    string    `gorm:"type:varchar(20);default:'active'" json:"status"` // active, pending, banned
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationships
	Group Group `gorm:"foreignKey:GroupID;references:ID" json:"group,omitempty"`
	User  User  `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (gm *GroupMember) BeforeCreate(tx *gorm.DB) error {
	if gm.ID == "" {
		gm.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name
func (GroupMember) TableName() string {
	return "group_members"
}

package repository

import (
	"errors"
	"time"

	"yourapp/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *model.User) error
	FindByID(id string) (*model.User, error)
	FindByEmail(email string) (*model.User, error)
	FindByUsername(username string) (*model.User, error)
	FindByGoogleID(googleID string) (*model.User, error)
	SearchUsers(keyword string, limit, offset int) ([]model.User, error)
	FindAll(limit, offset int) ([]model.User, int64, error) // Get all users with pagination
	Count() (int64, error)                                  // Count all users
	Update(user *model.User) error
	UpdateOTP(email string, otpCode string, expiresAt time.Time) error
	VerifyOTP(email string, otpCode string) (*model.User, error)
	UpdateResetToken(email string, token string, expiresAt time.Time) error
	FindByResetToken(token string) (*model.User, error)
	UpdatePassword(userID string, passwordHash string) error
	UpdateLastLogin(userID string) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindByID(id string) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByGoogleID(googleID string) (*model.User, error) {
	var user model.User
	err := r.db.Where("google_id = ?", googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SearchUsers searches users by keyword (name, username, email)
func (r *userRepository) SearchUsers(keyword string, limit, offset int) ([]model.User, error) {
	var users []model.User
	searchPattern := "%" + keyword + "%"

	query := r.db.Where("is_active = ?", true).
		Where("full_name ILIKE ? OR username ILIKE ? OR email ILIKE ?",
			searchPattern, searchPattern, searchPattern).
		Order("full_name ASC").
		Limit(limit).
		Offset(offset).
		Find(&users)

	if query.Error != nil {
		return nil, query.Error
	}

	return users, nil
}

func (r *userRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) UpdateOTP(email string, otpCode string, expiresAt time.Time) error {
	return r.db.Model(&model.User{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"otp_code":       otpCode,
			"otp_expires_at": expiresAt,
		}).Error
}

func (r *userRepository) VerifyOTP(email string, otpCode string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ? AND otp_code = ? AND otp_expires_at > ?", email, otpCode, time.Now()).First(&user).Error
	if err != nil {
		return nil, errors.New("invalid or expired OTP")
	}

	// Clear OTP after verification
	user.OTPCode = nil
	user.OTPExpiresAt = nil
	user.IsVerified = true

	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) UpdateResetToken(email string, token string, expiresAt time.Time) error {
	return r.db.Model(&model.User{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"reset_token":      token,
			"reset_expires_at": expiresAt,
		}).Error
}

func (r *userRepository) FindByResetToken(token string) (*model.User, error) {
	var user model.User
	err := r.db.Where("reset_token = ? AND reset_expires_at > ?", token, time.Now()).First(&user).Error
	if err != nil {
		return nil, errors.New("invalid or expired reset token")
	}
	return &user, nil
}

func (r *userRepository) UpdatePassword(userID string, passwordHash string) error {
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash":    passwordHash,
			"reset_token":      nil,
			"reset_expires_at": nil,
		}).Error
}

func (r *userRepository) UpdateLastLogin(userID string) error {
	now := time.Now()
	return r.db.Model(&model.User{}).
		Where("id = ?", userID).
		Update("last_login", now).Error
}

// FindAll gets all users with pagination
func (r *userRepository) FindAll(limit, offset int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	// Count total
	if err := r.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get users with pagination
	err := r.db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error

	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Count counts all users
func (r *userRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

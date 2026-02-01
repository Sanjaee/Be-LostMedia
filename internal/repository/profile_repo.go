package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type ProfileRepository interface {
	Create(profile *model.Profile) error
	FindByID(id string) (*model.Profile, error)
	FindByUserID(userID string) (*model.Profile, error)
	Update(profile *model.Profile) error
	Delete(id string) error
	DeleteByUserID(userID string) error
}

type profileRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	profileCachePrefix       = "profile:"
	profileByUserCachePrefix = "profile:user:"
	cacheExpiration          = 30 * time.Minute
)

func NewProfileRepository(db *gorm.DB, redis *util.RedisClient) ProfileRepository {
	return &profileRepository{
		db:    db,
		redis: redis,
	}
}

// getCacheKey returns the cache key for a profile by ID
func getCacheKey(id string) string {
	return profileCachePrefix + id
}

// getUserCacheKey returns the cache key for a profile by user ID
func getUserCacheKey(userID string) string {
	return profileByUserCachePrefix + userID
}

// Create creates a new profile and caches it
func (r *profileRepository) Create(profile *model.Profile) error {
	if err := r.db.Create(profile).Error; err != nil {
		return err
	}

	// Cache the profile
	if r.redis != nil {
		r.cacheProfile(profile)
	}

	return nil
}

// FindByID finds a profile by ID, checking cache first
func (r *profileRepository) FindByID(id string) (*model.Profile, error) {
	// Try to get from cache first
	if r.redis != nil {
		cached, err := r.getFromCache(getCacheKey(id))
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var profile model.Profile
	err := r.db.Preload("User").Where("id = ?", id).First(&profile).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheProfile(&profile)
	}

	return &profile, nil
}

// FindByUserID finds a profile by user ID, always loads fresh from DB to ensure User data
func (r *profileRepository) FindByUserID(userID string) (*model.Profile, error) {
	// Always load from DB to ensure User data is fresh and loaded
	// Cache might not include User data, so we bypass it for now
	var profile model.Profile
	err := r.db.Preload("User").Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, err
	}

	// Verify User data is loaded - if Preload failed, manually load user
	if profile.User.ID == "" || profile.User.ID != userID {
		var user model.User
		if err := r.db.Where("id = ?", userID).First(&user).Error; err == nil {
			profile.User = user
		} else {
			// User not found - this shouldn't happen but handle gracefully
			return nil, fmt.Errorf("user not found: %s", userID)
		}
	}

	// Cache the result (with User data)
	if r.redis != nil {
		r.cacheProfile(&profile)
	}

	return &profile, nil
}

// Update updates a profile and invalidates cache
func (r *profileRepository) Update(profile *model.Profile) error {
	if err := r.db.Save(profile).Error; err != nil {
		return err
	}

	// Invalidate and update cache
	if r.redis != nil {
		r.invalidateCache(profile.ID, profile.UserID)
		r.cacheProfile(profile)
	}

	return nil
}

// Delete deletes a profile and invalidates cache
func (r *profileRepository) Delete(id string) error {
	// Get profile first to get user_id for cache invalidation
	var profile model.Profile
	if err := r.db.Where("id = ?", id).First(&profile).Error; err != nil {
		return err
	}

	userID := profile.UserID

	// Delete from database
	if err := r.db.Delete(&profile).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateCache(id, userID)
	}

	return nil
}

// DeleteByUserID deletes a profile by user ID and invalidates cache
func (r *profileRepository) DeleteByUserID(userID string) error {
	// Delete from database
	result := r.db.Where("user_id = ?", userID).Delete(&model.Profile{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("profile not found")
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateCache("", userID)
	}

	return nil
}

// cacheProfile caches a profile with both ID and user ID keys
func (r *profileRepository) cacheProfile(profile *model.Profile) {
	if r.redis == nil {
		return
	}

	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return
	}

	// Cache by profile ID
	idKey := getCacheKey(profile.ID)
	r.redis.Set(idKey, string(profileJSON), cacheExpiration)

	// Cache by user ID
	userKey := getUserCacheKey(profile.UserID)
	r.redis.Set(userKey, string(profileJSON), cacheExpiration)
}

// getFromCache retrieves a profile from cache
func (r *profileRepository) getFromCache(key string) (*model.Profile, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var profile model.Profile
	if err := json.Unmarshal([]byte(cached), &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// invalidateCache removes profile from cache
func (r *profileRepository) invalidateCache(profileID, userID string) {
	if r.redis == nil {
		return
	}

	// Delete by profile ID
	if profileID != "" {
		r.redis.Delete(getCacheKey(profileID))
	}

	// Delete by user ID
	if userID != "" {
		r.redis.Delete(getUserCacheKey(userID))
	}
}

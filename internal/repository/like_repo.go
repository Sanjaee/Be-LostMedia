package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type LikeRepository interface {
	Create(like *model.Like) error
	Update(like *model.Like) error
	FindByID(id string) (*model.Like, error)
	FindByTarget(targetType, targetID string) ([]*model.Like, error)
	FindByUserAndTarget(userID, targetType, targetID string) (*model.Like, error)
	CountByTarget(targetType, targetID string) (int64, error)
	CountByTargets(targetType string, targetIDs []string) (map[string]int64, error)
	FindUserLikedTargets(userID, targetType string, targetIDs []string) (map[string]bool, error)
	Delete(id string) error
	DeleteByUserAndTarget(userID, targetType, targetID string) error
}

type likeRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	likeCachePrefix        = "like:"
	likeByTargetCachePrefix = "like:target:"
	likeCountCachePrefix   = "like:count:"
	likeCacheExpiration    = 10 * time.Minute
)

func NewLikeRepository(db *gorm.DB, redis *util.RedisClient) LikeRepository {
	return &likeRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new like and invalidates related caches
func (r *likeRepository) Create(like *model.Like) error {
	if err := r.db.Create(like).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateTargetCache(like.TargetType, like.TargetID)
		r.invalidateCountCache(like.TargetType, like.TargetID)
	}

	return nil
}

// Update updates a like and invalidates cache
func (r *likeRepository) Update(like *model.Like) error {
	if err := r.db.Save(like).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateTargetCache(like.TargetType, like.TargetID)
		r.invalidateCountCache(like.TargetType, like.TargetID)
	}

	return nil
}

// FindByID finds a like by ID
func (r *likeRepository) FindByID(id string) (*model.Like, error) {
	var like model.Like
	err := r.db.Preload("User").Where("id = ?", id).First(&like).Error
	if err != nil {
		return nil, err
	}
	return &like, nil
}

// FindByTarget finds all likes for a target (post or comment)
func (r *likeRepository) FindByTarget(targetType, targetID string) ([]*model.Like, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("%s%s:%s", likeByTargetCachePrefix, targetType, targetID)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var likes []*model.Like
	err := r.db.Preload("User").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		Find(&likes).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheLikeList(cacheKey, likes)
	}

	return likes, nil
}

// FindByUserAndTarget finds a like by user and target (to check if user already liked)
func (r *likeRepository) FindByUserAndTarget(userID, targetType, targetID string) (*model.Like, error) {
	var like model.Like
	err := r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).First(&like).Error
	if err != nil {
		return nil, err
	}
	return &like, nil
}

// CountByTarget counts likes for a target
func (r *likeRepository) CountByTarget(targetType, targetID string) (int64, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("%s%s:%s", likeCountCachePrefix, targetType, targetID)
	if r.redis != nil {
		cached, err := r.redis.Get(cacheKey)
		if err == nil {
			var count int64
			if _, err := fmt.Sscanf(cached, "%d", &count); err == nil {
				return count, nil
			}
		}
	}

	var count int64
	err := r.db.Model(&model.Like{}).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), likeCacheExpiration)
	}

	return count, nil
}

// CountByTargets counts likes for multiple targets in one query
func (r *likeRepository) CountByTargets(targetType string, targetIDs []string) (map[string]int64, error) {
	if len(targetIDs) == 0 {
		return map[string]int64{}, nil
	}
	var results []struct {
		TargetID string
		Count    int64
	}
	err := r.db.Model(&model.Like{}).
		Select("target_id, count(*) as count").
		Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Group("target_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, row := range results {
		m[row.TargetID] = row.Count
	}
	// Ensure all IDs have entry (0 if not found)
	for _, id := range targetIDs {
		if _, ok := m[id]; !ok {
			m[id] = 0
		}
	}
	return m, nil
}

// FindUserLikedTargets returns which targets the user has liked
func (r *likeRepository) FindUserLikedTargets(userID, targetType string, targetIDs []string) (map[string]bool, error) {
	if len(targetIDs) == 0 {
		return map[string]bool{}, nil
	}
	var likes []model.Like
	err := r.db.Select("target_id").
		Where("user_id = ? AND target_type = ? AND target_id IN ?", userID, targetType, targetIDs).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool)
	for _, like := range likes {
		m[like.TargetID] = true
	}
	return m, nil
}

// Delete deletes a like and invalidates cache
func (r *likeRepository) Delete(id string) error {
	// Get like first for cache invalidation
	var like model.Like
	if err := r.db.Where("id = ?", id).First(&like).Error; err != nil {
		return err
	}

	targetType := like.TargetType
	targetID := like.TargetID

	// Delete from database
	if err := r.db.Delete(&like).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateTargetCache(targetType, targetID)
		r.invalidateCountCache(targetType, targetID)
	}

	return nil
}

// DeleteByUserAndTarget deletes a like by user and target (for unlike functionality)
func (r *likeRepository) DeleteByUserAndTarget(userID, targetType, targetID string) error {
	// Get like first for cache invalidation
	var like model.Like
	if err := r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).First(&like).Error; err != nil {
		return err
	}

	// Delete from database
	if err := r.db.Delete(&like).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateTargetCache(targetType, targetID)
		r.invalidateCountCache(targetType, targetID)
	}

	return nil
}

// Cache helpers
func (r *likeRepository) cacheLikeList(key string, likes []*model.Like) {
	if r.redis == nil {
		return
	}

	likesJSON, err := json.Marshal(likes)
	if err != nil {
		return
	}

	r.redis.Set(key, string(likesJSON), likeCacheExpiration)
}

func (r *likeRepository) getListFromCache(key string) ([]*model.Like, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var likes []*model.Like
	if err := json.Unmarshal([]byte(cached), &likes); err != nil {
		return nil, err
	}

	return likes, nil
}

func (r *likeRepository) invalidateTargetCache(targetType, targetID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(fmt.Sprintf("%s%s:%s", likeByTargetCachePrefix, targetType, targetID))
}

func (r *likeRepository) invalidateCountCache(targetType, targetID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(fmt.Sprintf("%s%s:%s", likeCountCachePrefix, targetType, targetID))
}

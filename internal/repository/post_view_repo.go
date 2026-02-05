package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type PostViewRepository interface {
	CreateOrUpdate(view *model.PostView) error
	FindByPostID(postID string) ([]*model.PostView, error)
	FindByUserID(userID string, limit, offset int) ([]*model.PostView, error)
	FindByPostAndUser(postID, userID string) (*model.PostView, error)
	CountByPostID(postID string) (int64, error)
	DeleteByPostAndUser(postID, userID string) error
}

type postViewRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	postViewCachePrefix      = "post_view:"
	postViewByPostCachePrefix = "post_view:post:"
	postViewCountCachePrefix = "post_view:count:"
	postViewCacheExpiration  = 10 * time.Minute
)

func NewPostViewRepository(db *gorm.DB, redis *util.RedisClient) PostViewRepository {
	return &postViewRepository{
		db:    db,
		redis: redis,
	}
}

// CreateOrUpdate creates a new view or updates existing one (prevents duplicates)
func (r *postViewRepository) CreateOrUpdate(view *model.PostView) error {
	// Check if view already exists
	var existingView model.PostView
	err := r.db.Where("post_id = ? AND user_id = ?", view.PostID, view.UserID).First(&existingView).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new view
		if err := r.db.Create(view).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// View already exists, update timestamp (optional - you can skip this if you want truly unique)
		// For now, we'll just return success since view already exists
		return nil
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidatePostCache(view.PostID)
		r.invalidateCountCache(view.PostID)
	}

	return nil
}

// FindByPostID finds all views for a post
func (r *postViewRepository) FindByPostID(postID string) ([]*model.PostView, error) {
	// Try cache first
	cacheKey := postViewByPostCachePrefix + postID
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var views []*model.PostView
	err := r.db.Preload("User").
		Where("post_id = ?", postID).
		Order("created_at DESC").
		Find(&views).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheViewList(cacheKey, views)
	}

	return views, nil
}

// FindByUserID finds all views by a user
func (r *postViewRepository) FindByUserID(userID string, limit, offset int) ([]*model.PostView, error) {
	var views []*model.PostView
	err := r.db.Preload("Post").Preload("Post.User").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&views).Error
	if err != nil {
		return nil, err
	}
	return views, nil
}

// FindByPostAndUser finds a view by post and user
func (r *postViewRepository) FindByPostAndUser(postID, userID string) (*model.PostView, error) {
	var view model.PostView
	err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&view).Error
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// CountByPostID counts views for a post
func (r *postViewRepository) CountByPostID(postID string) (int64, error) {
	// Try cache first
	cacheKey := postViewCountCachePrefix + postID
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
	err := r.db.Model(&model.PostView{}).
		Where("post_id = ?", postID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), postViewCacheExpiration)
	}

	return count, nil
}

// DeleteByPostAndUser deletes a view
func (r *postViewRepository) DeleteByPostAndUser(postID, userID string) error {
	// Delete from database
	if err := r.db.Where("post_id = ? AND user_id = ?", postID, userID).Delete(&model.PostView{}).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidatePostCache(postID)
		r.invalidateCountCache(postID)
	}

	return nil
}

// Cache helpers
func (r *postViewRepository) cacheViewList(key string, views []*model.PostView) {
	if r.redis == nil {
		return
	}

	viewsJSON, err := json.Marshal(views)
	if err != nil {
		return
	}

	r.redis.Set(key, string(viewsJSON), postViewCacheExpiration)
}

func (r *postViewRepository) getListFromCache(key string) ([]*model.PostView, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var views []*model.PostView
	if err := json.Unmarshal([]byte(cached), &views); err != nil {
		return nil, err
	}

	return views, nil
}

func (r *postViewRepository) invalidatePostCache(postID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(postViewByPostCachePrefix + postID)
}

func (r *postViewRepository) invalidateCountCache(postID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(postViewCountCachePrefix + postID)
}

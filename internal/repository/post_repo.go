package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type PostRepository interface {
	Create(post *model.Post) error
	FindByID(id string) (*model.Post, error)
	FindByUserID(userID string, limit, offset int) ([]*model.Post, error)
	FindByGroupID(groupID string, limit, offset int) ([]*model.Post, error)
	FindFeed(userID string, limit, offset int) ([]*model.Post, error) // Feed for user (friends' posts + own posts)
	Update(post *model.Post) error
	Delete(id string) error
	CountByUserID(userID string) (int64, error)
	CountByGroupID(groupID string) (int64, error)
}

type postRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	postCachePrefix        = "post:"
	postByUserCachePrefix  = "post:user:"
	postByGroupCachePrefix = "post:group:"
	postFeedCachePrefix    = "post:feed:"
	postCountCachePrefix   = "post:count:"
	postCacheExpiration    = 15 * time.Minute
)

func NewPostRepository(db *gorm.DB, redis *util.RedisClient) PostRepository {
	return &postRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new post and invalidates related caches
func (r *postRepository) Create(post *model.Post) error {
	if err := r.db.Create(post).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateUserCache(post.UserID)
		if post.GroupID != nil {
			r.invalidateGroupCache(*post.GroupID)
		}
		r.invalidateFeedCache(post.UserID)
		r.invalidateCountCache(post.UserID)
		if post.GroupID != nil {
			r.invalidateGroupCountCache(*post.GroupID)
		}
	}

	return nil
}

// FindByID finds a post by ID, checking cache first
func (r *postRepository) FindByID(id string) (*model.Post, error) {
	// Try to get from cache first
	if r.redis != nil {
		cached, err := r.getFromCache(postCachePrefix + id)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var post model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
		Where("id = ?", id).First(&post).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cachePost(&post)
	}

	return &post, nil
}

// FindByUserID finds posts by user ID, checking cache first
func (r *postRepository) FindByUserID(userID string, limit, offset int) ([]*model.Post, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", postByUserCachePrefix, userID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var posts []*model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&posts).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cachePostList(cacheKey, posts)
	}

	return posts, nil
}

// FindByGroupID finds posts by group ID, checking cache first
func (r *postRepository) FindByGroupID(groupID string, limit, offset int) ([]*model.Post, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", postByGroupCachePrefix, groupID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var posts []*model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
		Where("group_id = ?", groupID).
		Order("is_pinned DESC, created_at DESC").
		Limit(limit).Offset(offset).
		Find(&posts).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cachePostList(cacheKey, posts)
	}

	return posts, nil
}

// FindFeed finds feed posts for a user (friends' posts + own posts)
// This is a simplified version - in production, you'd want to filter by privacy settings
func (r *postRepository) FindFeed(userID string, limit, offset int) ([]*model.Post, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", postFeedCachePrefix, userID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Get user's friends (accepted friendships)
	var friendships []*model.Friendship
	if err := r.db.Where("(sender_id = ? OR receiver_id = ?) AND status = ?", userID, userID, model.FriendshipStatusAccepted).
		Find(&friendships).Error; err != nil {
		return nil, err
	}

	// Collect friend IDs
	friendIDs := []string{userID} // Include own posts
	for _, f := range friendships {
		if f.SenderID == userID {
			friendIDs = append(friendIDs, f.ReceiverID)
		} else {
			friendIDs = append(friendIDs, f.SenderID)
		}
	}

	// Get posts from friends and own posts
	// For now, we'll show public and friends posts
	var posts []*model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
		Where("user_id IN ? AND (privacy = ? OR privacy = ?)", friendIDs, model.PrivacyPublic, model.PrivacyFriends).
		Where("group_id IS NULL"). // Exclude group posts from feed for now
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&posts).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cachePostList(cacheKey, posts)
	}

	return posts, nil
}

// Update updates a post and invalidates cache
func (r *postRepository) Update(post *model.Post) error {
	if err := r.db.Save(post).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidatePostCache(post.ID)
		r.invalidateUserCache(post.UserID)
		if post.GroupID != nil {
			r.invalidateGroupCache(*post.GroupID)
		}
		r.invalidateFeedCache(post.UserID)
	}

	return nil
}

// Delete deletes a post and invalidates cache
func (r *postRepository) Delete(id string) error {
	// Get post first for cache invalidation
	var post model.Post
	if err := r.db.Where("id = ?", id).First(&post).Error; err != nil {
		return err
	}

	userID := post.UserID
	groupID := post.GroupID

	// Delete from database (soft delete)
	if err := r.db.Delete(&post).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidatePostCache(id)
		r.invalidateUserCache(userID)
		if groupID != nil {
			r.invalidateGroupCache(*groupID)
		}
		r.invalidateFeedCache(userID)
		r.invalidateCountCache(userID)
		if groupID != nil {
			r.invalidateGroupCountCache(*groupID)
		}
	}

	return nil
}

// CountByUserID counts posts by user ID
func (r *postRepository) CountByUserID(userID string) (int64, error) {
	// Try cache first
	cacheKey := postCountCachePrefix + "user:" + userID
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
	err := r.db.Model(&model.Post{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), postCacheExpiration)
	}

	return count, nil
}

// CountByGroupID counts posts by group ID
func (r *postRepository) CountByGroupID(groupID string) (int64, error) {
	// Try cache first
	cacheKey := postCountCachePrefix + "group:" + groupID
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
	err := r.db.Model(&model.Post{}).Where("group_id = ?", groupID).Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), postCacheExpiration)
	}

	return count, nil
}

// Cache helpers
func (r *postRepository) cachePost(post *model.Post) {
	if r.redis == nil {
		return
	}

	postJSON, err := json.Marshal(post)
	if err != nil {
		return
	}

	r.redis.Set(postCachePrefix+post.ID, string(postJSON), postCacheExpiration)
}

func (r *postRepository) cachePostList(key string, posts []*model.Post) {
	if r.redis == nil {
		return
	}

	postsJSON, err := json.Marshal(posts)
	if err != nil {
		return
	}

	r.redis.Set(key, string(postsJSON), postCacheExpiration)
}

func (r *postRepository) getFromCache(key string) (*model.Post, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var post model.Post
	if err := json.Unmarshal([]byte(cached), &post); err != nil {
		return nil, err
	}

	return &post, nil
}

func (r *postRepository) getListFromCache(key string) ([]*model.Post, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var posts []*model.Post
	if err := json.Unmarshal([]byte(cached), &posts); err != nil {
		return nil, err
	}

	return posts, nil
}

func (r *postRepository) invalidatePostCache(id string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(postCachePrefix + id)
	// Also invalidate list caches that might contain this post
	r.redis.DeletePattern(postByUserCachePrefix + "*")
	r.redis.DeletePattern(postByGroupCachePrefix + "*")
	r.redis.DeletePattern(postFeedCachePrefix + "*")
}

func (r *postRepository) invalidateUserCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(postByUserCachePrefix + userID + ":*")
	r.redis.DeletePattern(postFeedCachePrefix + userID + ":*")
}

func (r *postRepository) invalidateGroupCache(groupID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(postByGroupCachePrefix + groupID + ":*")
}

func (r *postRepository) invalidateFeedCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(postFeedCachePrefix + userID + ":*")
	// Also invalidate friends' feed caches
	r.redis.DeletePattern(postFeedCachePrefix + "*")
}

func (r *postRepository) invalidateCountCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(postCountCachePrefix + "user:" + userID)
}

func (r *postRepository) invalidateGroupCountCache(groupID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(postCountCachePrefix + "group:" + groupID)
}

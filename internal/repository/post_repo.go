package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type PostRepository interface {
	Create(post *model.Post) error
	FindByID(id string) (*model.Post, error)
	FindByUserID(userID string, limit, offset int) ([]*model.Post, error)
	FindByGroupID(groupID string, limit, offset int) ([]*model.Post, error)
	FindFeed(userID string, limit, offset int) ([]*model.Post, error)             // Feed for user (friends' posts + own posts)
	FindFeedByEngagement(userID string, limit, offset int) ([]*model.Post, error) // Feed sorted by engagement (likes + comments + views)
	Update(post *model.Post) error
	Delete(id string) error
	CountByUserID(userID string) (int64, error)
	CountByGroupID(groupID string) (int64, error)
	UpdatePostEngagementScore(postID string) // Update engagement score in Redis
}

type postRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	postCachePrefix               = "post:"
	postByUserCachePrefix         = "post:user:"
	postByGroupCachePrefix        = "post:group:"
	postFeedCachePrefix           = "post:feed:"
	postCountCachePrefix          = "post:count:"
	postEngagementScorePrefix     = "post:engagement:score:" // Individual post engagement score
	postEngagementSortedSetKey    = "post:engagement:sorted" // Sorted set of all posts by engagement
	postCacheExpiration           = 15 * time.Minute
	postEngagementCacheExpiration = 30 * time.Minute // Longer cache for engagement scores
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
		// Add to engagement sorted set with initial score 0 (only for non-group posts)
		if post.GroupID == nil {
			tieBreakScore := float64(post.CreatedAt.Unix()) / 1000000.0
			r.redis.ZAdd(postEngagementSortedSetKey, tieBreakScore, post.ID)
			r.redis.Set(postEngagementScorePrefix+post.ID, "0", postEngagementCacheExpiration)
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

// FindFeed finds feed posts for a user (all posts from all users - all posts are public)
func (r *postRepository) FindFeed(userID string, limit, offset int) ([]*model.Post, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", postFeedCachePrefix, userID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Get all posts (all posts are public now)
	var posts []*model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
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

// FindFeedByEngagement finds feed posts sorted by engagement score (likes + comments + views)
// Uses Redis sorted set for fast sorting
func (r *postRepository) FindFeedByEngagement(userID string, limit, offset int) ([]*model.Post, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s%s:engagement:%d:%d", postFeedCachePrefix, userID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Try to get sorted post IDs from Redis sorted set
	var postIDs []string
	if r.redis != nil {
		// Get post IDs sorted by engagement score (descending)
		ids, err := r.redis.ZRevRange(postEngagementSortedSetKey, int64(offset), int64(offset+limit-1))
		if err == nil && len(ids) > 0 {
			postIDs = ids
		}
	}

	// If Redis sorted set is empty or not available, build it
	if len(postIDs) == 0 {
		// Get all posts first
		var allPosts []*model.Post
		err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
			Preload("Tags.TaggedUser").Preload("Location").
			Where("group_id IS NULL").
			Find(&allPosts).Error
		if err != nil {
			return nil, err
		}

		// Calculate and cache engagement scores
		if r.redis != nil {
			// Use pipeline for batch operations
			ctx := context.Background()
			pipe := r.redis.GetClient().Pipeline()

			for _, post := range allPosts {
				// Try to get cached engagement score first
				scoreKey := postEngagementScorePrefix + post.ID
				cachedScore, err := r.redis.Get(scoreKey)
				var score float64

				if err != nil {
					// Calculate engagement score (MUST calculate before returning)
					var likeCount, commentCount, viewCount int64

					// Get counts - these queries MUST complete before we return
					r.db.Model(&model.Like{}).
						Where("target_type = ? AND target_id = ?", "post", post.ID).
						Count(&likeCount)
					r.db.Model(&model.Comment{}).
						Where("post_id = ?", post.ID).
						Count(&commentCount)
					r.db.Model(&model.PostView{}).
						Where("post_id = ?", post.ID).
						Count(&viewCount)

					// Calculate engagement score
					// Weight: likes = 2, comments = 3, views = 1
					score = float64((likeCount * 2) + (commentCount * 3) + (viewCount * 1))

					// Cache the score
					pipe.Set(ctx, scoreKey, fmt.Sprintf("%.0f", score), postEngagementCacheExpiration)
				} else {
					// Parse cached score
					fmt.Sscanf(cachedScore, "%f", &score)
				}

				// Add to sorted set with score
				// Use negative timestamp for tie-breaking (newer posts have higher priority)
				tieBreakScore := float64(post.CreatedAt.Unix()) / 1000000.0 // Normalize to small value
				finalScore := score*1000000.0 + tieBreakScore               // Multiply by large number to prioritize engagement
				pipe.ZAdd(ctx, postEngagementSortedSetKey, redis.Z{
					Score:  finalScore,
					Member: post.ID,
				})
			}

			// Set expiration for sorted set
			pipe.Expire(ctx, postEngagementSortedSetKey, postEngagementCacheExpiration)

			// Execute pipeline - MUST wait for completion before returning
			_, err = pipe.Exec(ctx)
			if err != nil {
				// If Redis fails, fall back to database (which also calculates scores)
				log.Printf("Redis pipeline error: %v, falling back to DB", err)
			} else {
				// Get sorted post IDs from Redis (after scores are calculated)
				ids, err := r.redis.ZRevRange(postEngagementSortedSetKey, int64(offset), int64(offset+limit-1))
				if err == nil && len(ids) > 0 {
					postIDs = ids
				}
			}
		}

		// If still no post IDs, fall back to in-memory sorting
		if len(postIDs) == 0 {
			return r.findFeedByEngagementFallback(allPosts, limit, offset)
		}
	}

	// Load posts by IDs from database
	var posts []*model.Post
	err := r.db.Preload("User").Preload("Group").Preload("SharedPost").
		Preload("Tags.TaggedUser").Preload("Location").
		Where("id IN ? AND group_id IS NULL", postIDs).
		Find(&posts).Error
	if err != nil {
		return nil, err
	}

	// Sort posts to match the order from Redis
	postMap := make(map[string]*model.Post)
	for _, post := range posts {
		postMap[post.ID] = post
	}

	result := make([]*model.Post, 0, len(postIDs))
	for _, id := range postIDs {
		if post, ok := postMap[id]; ok {
			result = append(result, post)
		}
	}

	// Cache the result
	if r.redis != nil {
		r.cachePostList(cacheKey, result)
	}

	return result, nil
}

// findFeedByEngagementFallback is fallback method when Redis is not available
func (r *postRepository) findFeedByEngagementFallback(posts []*model.Post, limit, offset int) ([]*model.Post, error) {
	type PostWithScore struct {
		Post  *model.Post
		Score int64
	}

	postsWithScore := make([]PostWithScore, 0, len(posts))

	for _, post := range posts {
		var likeCount, commentCount, viewCount int64

		r.db.Model(&model.Like{}).
			Where("target_type = ? AND target_id = ?", "post", post.ID).
			Count(&likeCount)
		r.db.Model(&model.Comment{}).
			Where("post_id = ?", post.ID).
			Count(&commentCount)
		r.db.Model(&model.PostView{}).
			Where("post_id = ?", post.ID).
			Count(&viewCount)

		score := (likeCount * 2) + (commentCount * 3) + (viewCount * 1)

		postsWithScore = append(postsWithScore, PostWithScore{
			Post:  post,
			Score: score,
		})
	}

	// Use sort.Slice for better performance
	sort.Slice(postsWithScore, func(i, j int) bool {
		if postsWithScore[i].Score != postsWithScore[j].Score {
			return postsWithScore[i].Score > postsWithScore[j].Score
		}
		return postsWithScore[i].Post.CreatedAt.After(postsWithScore[j].Post.CreatedAt)
	})

	result := make([]*model.Post, 0, limit)
	start := offset
	end := offset + limit
	if start > len(postsWithScore) {
		return []*model.Post{}, nil
	}
	if end > len(postsWithScore) {
		end = len(postsWithScore)
	}

	for i := start; i < end; i++ {
		result = append(result, postsWithScore[i].Post)
	}

	return result, nil
}

// UpdatePostEngagementScore updates the engagement score for a post in Redis
// This is called when likes, comments, or views change
func (r *postRepository) UpdatePostEngagementScore(postID string) {
	if r.redis == nil {
		return
	}

	// Get current counts
	var likeCount, commentCount, viewCount int64
	r.db.Model(&model.Like{}).
		Where("target_type = ? AND target_id = ?", "post", postID).
		Count(&likeCount)
	r.db.Model(&model.Comment{}).
		Where("post_id = ?", postID).
		Count(&commentCount)
	r.db.Model(&model.PostView{}).
		Where("post_id = ?", postID).
		Count(&viewCount)

	// Calculate engagement score
	score := float64((likeCount * 2) + (commentCount * 3) + (viewCount * 1))

	// Get post for created_at
	var post model.Post
	if err := r.db.Where("id = ?", postID).First(&post).Error; err != nil {
		return
	}

	// Update cached score
	scoreKey := postEngagementScorePrefix + postID
	r.redis.Set(scoreKey, fmt.Sprintf("%.0f", score), postEngagementCacheExpiration)

	// Update sorted set
	tieBreakScore := float64(post.CreatedAt.Unix()) / 1000000.0
	finalScore := score*1000000.0 + tieBreakScore
	r.redis.ZAdd(postEngagementSortedSetKey, finalScore, postID)
}

// Update updates a post and updates cache instead of invalidating
func (r *postRepository) Update(post *model.Post) error {
	if err := r.db.Save(post).Error; err != nil {
		return err
	}

	// Update caches instead of invalidating (keeps cache warm)
	if r.redis != nil {
		// Update post cache
		r.cachePost(post)

		// Update engagement score if needed (content change doesn't affect engagement, but we keep it fresh)
		// Only update if post is in sorted set (non-group posts)
		if post.GroupID == nil {
			r.UpdatePostEngagementScore(post.ID)
		}

		// Note: We don't invalidate feed/user caches to keep them warm
		// The feed will be updated naturally through engagement score updates
	}

	return nil
}

// Delete deletes a post and removes from cache (only remove from sorted set, keep other caches)
func (r *postRepository) Delete(id string) error {
	// Get post first for cache operations
	var post model.Post
	if err := r.db.Where("id = ?", id).First(&post).Error; err != nil {
		return err
	}

	groupID := post.GroupID

	// Delete from database (soft delete)
	if err := r.db.Delete(&post).Error; err != nil {
		return err
	}

	// Update caches instead of invalidating (keep cache warm)
	if r.redis != nil {
		// Remove post from individual cache
		r.redis.Delete(postCachePrefix + id)

		// Remove from engagement sorted set (only for non-group posts)
		if groupID == nil {
			r.redis.ZRem(postEngagementSortedSetKey, id)
			r.redis.Delete(postEngagementScorePrefix + id)
		}

		// Note: We don't invalidate feed/user caches to keep them warm
		// The feed will naturally exclude deleted posts on next fetch
		// Count caches will be updated on next count operation
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

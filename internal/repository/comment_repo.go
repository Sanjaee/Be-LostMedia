package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type CommentRepository interface {
	Create(comment *model.Comment) error
	FindByID(id string) (*model.Comment, error)
	FindByPostID(postID string, limit, offset int) ([]*model.Comment, error)
	FindByParentID(parentID string, limit, offset int) ([]*model.Comment, error)
	Update(comment *model.Comment) error
	Delete(id string) error
	CountByPostID(postID string) (int64, error)
	CountByPostIDs(postIDs []string) (map[string]int64, error)
	CountByParentID(parentID string) (int64, error)
}

type commentRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	commentCachePrefix         = "comment:"
	commentByPostCachePrefix   = "comment:post:"
	commentByParentCachePrefix = "comment:parent:"
	commentCountCachePrefix    = "comment:count:"
	commentCacheExpiration     = 15 * time.Minute
)

func NewCommentRepository(db *gorm.DB, redis *util.RedisClient) CommentRepository {
	return &commentRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new comment and invalidates related caches
func (r *commentRepository) Create(comment *model.Comment) error {
	if err := r.db.Create(comment).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidatePostCache(comment.PostID)
		r.invalidateCountCache(comment.PostID)
		if comment.ParentID != nil {
			r.invalidateParentCache(*comment.ParentID)
			r.invalidateParentCountCache(*comment.ParentID)
		}
	}

	return nil
}

// FindByID finds a comment by ID
func (r *commentRepository) FindByID(id string) (*model.Comment, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getFromCache(commentCachePrefix + id)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var comment model.Comment
	err := r.db.Preload("User").Preload("Parent").Preload("Parent.User").
		Where("id = ?", id).First(&comment).Error
	if err != nil {
		return nil, err
	}

	// Load replies recursively
	r.loadRepliesRecursive(&comment)

	// Cache the result
	if r.redis != nil {
		r.cacheComment(&comment)
	}

	return &comment, nil
}

// FindByPostID finds comments by post ID with all nested replies
func (r *commentRepository) FindByPostID(postID string, limit, offset int) ([]*model.Comment, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", commentByPostCachePrefix, postID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Get top-level comments only (parent_id IS NULL)
	var comments []*model.Comment
	err := r.db.Preload("User").
		Where("post_id = ? AND parent_id IS NULL", postID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&comments).Error
	if err != nil {
		return nil, err
	}

	// Load nested replies recursively for each top-level comment
	for i := range comments {
		r.loadRepliesRecursive(comments[i])
		r.loadLikeCountRecursive(comments[i])
	}

	// Cache the result
	if r.redis != nil {
		r.cacheCommentList(cacheKey, comments)
	}

	return comments, nil
}

// loadRepliesRecursive loads all nested replies for a comment recursively
func (r *commentRepository) loadRepliesRecursive(comment *model.Comment) {
	var replies []model.Comment
	err := r.db.Preload("User").
		Preload("Parent").
		Preload("Parent.User").
		Where("parent_id = ?", comment.ID).
		Order("created_at ASC").
		Find(&replies).Error
	
	if err != nil || len(replies) == 0 {
		return
	}

	// Recursively load replies for each reply
	for i := range replies {
		r.loadRepliesRecursive(&replies[i])
	}

	comment.Replies = replies
}

// loadLikeCountRecursive loads like counts for comment and all nested replies
func (r *commentRepository) loadLikeCountRecursive(comment *model.Comment) {
	// Load like count for this comment
	var likeCount int64
	r.db.Model(&model.Like{}).
		Where("target_type = ? AND target_id = ?", model.TargetTypeComment, comment.ID).
		Count(&likeCount)
	comment.LikeCount = likeCount

	// Recursively load like counts for all replies
	for i := range comment.Replies {
		r.loadLikeCountRecursive(&comment.Replies[i])
	}
}

// FindByParentID finds replies to a comment
func (r *commentRepository) FindByParentID(parentID string, limit, offset int) ([]*model.Comment, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("%s%s:%d:%d", commentByParentCachePrefix, parentID, limit, offset)
	if r.redis != nil {
		cached, err := r.getListFromCache(cacheKey)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// If not in cache, get from database
	var comments []*model.Comment
	err := r.db.Preload("User").
		Preload("Parent").
		Preload("Parent.User").
		Where("parent_id = ?", parentID).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&comments).Error
	if err != nil {
		return nil, err
	}

	// Load nested replies and like counts for each comment
	for i := range comments {
		r.loadRepliesRecursive(comments[i])
		r.loadLikeCountRecursive(comments[i])
	}

	// Cache the result
	if r.redis != nil {
		r.cacheCommentList(cacheKey, comments)
	}

	return comments, nil
}

// Update updates a comment and invalidates cache
func (r *commentRepository) Update(comment *model.Comment) error {
	if err := r.db.Save(comment).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateCommentCache(comment.ID)
		r.invalidatePostCache(comment.PostID)
		if comment.ParentID != nil {
			r.invalidateParentCache(*comment.ParentID)
		}
	}

	return nil
}

// Delete deletes a comment (soft delete) and invalidates cache
func (r *commentRepository) Delete(id string) error {
	// Get comment first for cache invalidation
	var comment model.Comment
	if err := r.db.Where("id = ?", id).First(&comment).Error; err != nil {
		return err
	}

	postID := comment.PostID
	parentID := comment.ParentID

	// Delete from database (soft delete)
	if err := r.db.Delete(&comment).Error; err != nil {
		return err
	}

	// Invalidate caches
	if r.redis != nil {
		r.invalidateCommentCache(id)
		r.invalidatePostCache(postID)
		r.invalidateCountCache(postID)
		if parentID != nil {
			r.invalidateParentCache(*parentID)
			r.invalidateParentCountCache(*parentID)
		}
	}

	return nil
}

// CountByPostID counts comments by post ID
func (r *commentRepository) CountByPostID(postID string) (int64, error) {
	// Try cache first
	cacheKey := commentCountCachePrefix + "post:" + postID
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
	err := r.db.Model(&model.Comment{}).
		Where("post_id = ?", postID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), commentCacheExpiration)
	}

	return count, nil
}

// CountByPostIDs counts comments for multiple posts in one query (includes replies)
func (r *commentRepository) CountByPostIDs(postIDs []string) (map[string]int64, error) {
	if len(postIDs) == 0 {
		return map[string]int64{}, nil
	}
	var results []struct {
		PostID string
		Count  int64
	}
	err := r.db.Model(&model.Comment{}).
		Select("post_id, count(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, row := range results {
		m[row.PostID] = row.Count
	}
	for _, id := range postIDs {
		if _, ok := m[id]; !ok {
			m[id] = 0
		}
	}
	return m, nil
}

// CountByParentID counts replies to a comment
func (r *commentRepository) CountByParentID(parentID string) (int64, error) {
	// Try cache first
	cacheKey := commentCountCachePrefix + "parent:" + parentID
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
	err := r.db.Model(&model.Comment{}).
		Where("parent_id = ?", parentID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(cacheKey, fmt.Sprintf("%d", count), commentCacheExpiration)
	}

	return count, nil
}

// Cache helpers
func (r *commentRepository) cacheComment(comment *model.Comment) {
	if r.redis == nil {
		return
	}

	commentJSON, err := json.Marshal(comment)
	if err != nil {
		return
	}

	r.redis.Set(commentCachePrefix+comment.ID, string(commentJSON), commentCacheExpiration)
}

func (r *commentRepository) cacheCommentList(key string, comments []*model.Comment) {
	if r.redis == nil {
		return
	}

	commentsJSON, err := json.Marshal(comments)
	if err != nil {
		return
	}

	r.redis.Set(key, string(commentsJSON), commentCacheExpiration)
}

func (r *commentRepository) getFromCache(key string) (*model.Comment, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var comment model.Comment
	if err := json.Unmarshal([]byte(cached), &comment); err != nil {
		return nil, err
	}

	return &comment, nil
}

func (r *commentRepository) getListFromCache(key string) ([]*model.Comment, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var comments []*model.Comment
	if err := json.Unmarshal([]byte(cached), &comments); err != nil {
		return nil, err
	}

	return comments, nil
}

func (r *commentRepository) invalidateCommentCache(id string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(commentCachePrefix + id)
}

func (r *commentRepository) invalidatePostCache(postID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(commentByPostCachePrefix + postID + ":*")
	r.redis.Delete(commentCountCachePrefix + "post:" + postID)
}

func (r *commentRepository) invalidateParentCache(parentID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(commentByParentCachePrefix + parentID + ":*")
}

func (r *commentRepository) invalidateCountCache(postID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(commentCountCachePrefix + "post:" + postID)
}

func (r *commentRepository) invalidateParentCountCache(parentID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(commentCountCachePrefix + "parent:" + parentID)
}
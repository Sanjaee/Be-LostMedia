package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type PostService interface {
	CreatePost(userID string, req CreatePostRequest) (*model.Post, error)
	GetPostByID(postID string, viewerID string) (*model.Post, error)
	GetPostsByUserID(userID string, viewerID string, limit, offset int) ([]*model.Post, error)
	GetPostsByGroupID(groupID string, viewerID string, limit, offset int) ([]*model.Post, error)
	GetFeed(userID string, limit, offset int) ([]*model.Post, error)
	GetFeedByEngagement(userID string, limit, offset int) ([]*model.Post, error)
	UpdatePost(userID string, postID string, req UpdatePostRequest) (*model.Post, error)
	DeletePost(userID string, postID string) error
	CountPostsByUserID(userID string) (int64, error)
	CountPostsByGroupID(groupID string) (int64, error)
}

type postService struct {
	postRepo       repository.PostRepository
	userRepo       repository.UserRepository
	friendshipRepo repository.FriendshipRepository
}

type CreatePostRequest struct {
	Content      *string                `json:"content,omitempty"`
	ImageURLs    []string               `json:"image_urls,omitempty"` // Array of image URLs
	SharedPostID *string                `json:"shared_post_id,omitempty"`
	GroupID      *string                `json:"group_id,omitempty"`
	IsPinned     *bool                  `json:"is_pinned,omitempty"`
	Tags         []string               `json:"tags,omitempty"` // Array of user IDs to tag
	Location     *CreateLocationRequest `json:"location,omitempty"`
}

type CreateLocationRequest struct {
	PlaceName *string  `json:"place_name,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type UpdatePostRequest struct {
	Content   *string  `json:"content,omitempty"`
	ImageURLs []string `json:"image_urls,omitempty"` // Array of image URLs
	IsPinned  *bool    `json:"is_pinned,omitempty"`
}

func NewPostService(
	postRepo repository.PostRepository,
	userRepo repository.UserRepository,
	friendshipRepo repository.FriendshipRepository,
) PostService {
	return &postService{
		postRepo:       postRepo,
		userRepo:       userRepo,
		friendshipRepo: friendshipRepo,
	}
}

// CreatePost creates a new post
func (s *postService) CreatePost(userID string, req CreatePostRequest) (*model.Post, error) {
	// Validate user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	// Validate shared post if provided
	if req.SharedPostID != nil {
		if _, err := s.postRepo.FindByID(*req.SharedPostID); err != nil {
			return nil, errors.New("shared post not found")
		}
	}

	// Serialize ImageURLs array to JSON string
	// For empty array, use empty JSON array "[]" instead of empty string
	// PostgreSQL JSONB requires valid JSON or NULL
	var imageURLsJSON string
	if len(req.ImageURLs) > 0 {
		imageURLsBytes, err := json.Marshal(req.ImageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize image URLs: %w", err)
		}
		imageURLsJSON = string(imageURLsBytes)
	} else {
		// Use empty JSON array for empty image URLs
		imageURLsJSON = "[]"
	}

	// Create post
	post := &model.Post{
		UserID:       userID,
		Content:      req.Content,
		ImageURLs:    imageURLsJSON,
		SharedPostID: req.SharedPostID,
		GroupID:      req.GroupID,
		IsPinned:     false,
	}

	if req.IsPinned != nil {
		post.IsPinned = *req.IsPinned
	}

	// Validate: must have either content or image URLs
	if (req.Content == nil || *req.Content == "") && len(req.ImageURLs) == 0 {
		return nil, errors.New("post must have either content or image URLs")
	}

	if err := s.postRepo.Create(post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// Reload with relationships
	return s.postRepo.FindByID(post.ID)
}

// GetPostByID retrieves a post by ID (all posts are public)
func (s *postService) GetPostByID(postID string, viewerID string) (*model.Post, error) {
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}
	return post, nil
}

// GetPostsByUserID retrieves posts by user ID (all posts are public)
func (s *postService) GetPostsByUserID(userID string, viewerID string, limit, offset int) ([]*model.Post, error) {
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	posts, err := s.postRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	return posts, nil
}

// GetPostsByGroupID retrieves posts by group ID
func (s *postService) GetPostsByGroupID(groupID string, viewerID string, limit, offset int) ([]*model.Post, error) {
	posts, err := s.postRepo.FindByGroupID(groupID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	// For now, we'll return all posts (group membership check will be added later)
	return posts, nil
}

// GetFeed retrieves feed posts for a user (sorted by newest first)
func (s *postService) GetFeed(userID string, limit, offset int) ([]*model.Post, error) {
	// Check if user exists
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	posts, err := s.postRepo.FindFeed(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}

	return posts, nil
}

// GetFeedByEngagement retrieves feed posts sorted by engagement (likes + comments + views)
func (s *postService) GetFeedByEngagement(userID string, limit, offset int) ([]*model.Post, error) {
	// Check if user exists
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	posts, err := s.postRepo.FindFeedByEngagement(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed by engagement: %w", err)
	}

	return posts, nil
}

// UpdatePost updates a post
func (s *postService) UpdatePost(userID string, postID string, req UpdatePostRequest) (*model.Post, error) {
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	if post.UserID != userID {
		return nil, errors.New("unauthorized: you can only update your own posts")
	}

	// Update fields
	if req.Content != nil {
		post.Content = req.Content
	}
	if req.ImageURLs != nil {
		if len(req.ImageURLs) > 0 {
			imageURLsBytes, err := json.Marshal(req.ImageURLs)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize image URLs: %w", err)
			}
			post.ImageURLs = string(imageURLsBytes)
		} else {
			// Use empty JSON array for PostgreSQL JSONB compatibility
			post.ImageURLs = "[]"
		}
	}
	if req.IsPinned != nil {
		post.IsPinned = *req.IsPinned
	}

	if err := s.postRepo.Update(post); err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	return s.postRepo.FindByID(post.ID)
}

// DeletePost deletes a post (owner or admin can delete)
func (s *postService) DeletePost(userID string, postID string) error {
	// Get existing post
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return errors.New("post not found")
	}

	// Check if user owns this post
	if post.UserID != userID {
		// If not owner, check if user is owner role
		user, err := s.userRepo.FindByID(userID)
		if err != nil {
			return errors.New("user not found")
		}
		
		// Only owner can delete other users' posts
		if user.UserType != "owner" {
			return errors.New("unauthorized: you can only delete your own posts")
		}
	}

	if err := s.postRepo.Delete(postID); err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	return nil
}

// CountPostsByUserID counts posts by user ID
func (s *postService) CountPostsByUserID(userID string) (int64, error) {
	return s.postRepo.CountByUserID(userID)
}

// CountPostsByGroupID counts posts by group ID
func (s *postService) CountPostsByGroupID(groupID string) (int64, error) {
	return s.postRepo.CountByGroupID(groupID)
}

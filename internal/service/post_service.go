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
	Privacy      string                 `json:"privacy"` // public, friends, only_me
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
	Privacy   *string  `json:"privacy,omitempty"`
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
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Validate privacy
	privacy := model.PrivacyPublic
	if req.Privacy != "" {
		validPrivacy := map[string]bool{
			model.PrivacyPublic:  true,
			model.PrivacyFriends: true,
			model.PrivacyOnlyMe:  true,
		}
		if !validPrivacy[req.Privacy] {
			return nil, errors.New("invalid privacy setting")
		}
		privacy = req.Privacy
	}

	// Validate shared post if provided
	if req.SharedPostID != nil {
		sharedPost, err := s.postRepo.FindByID(*req.SharedPostID)
		if err != nil {
			return nil, errors.New("shared post not found")
		}
		// Check if shared post is accessible
		if !s.canViewPost(sharedPost, userID) {
			return nil, errors.New("cannot share this post")
		}
	}

	// Serialize ImageURLs array to JSON string
	var imageURLsJSON string
	if len(req.ImageURLs) > 0 {
		imageURLsBytes, err := json.Marshal(req.ImageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize image URLs: %w", err)
		}
		imageURLsJSON = string(imageURLsBytes)
	}

	// Create post
	post := &model.Post{
		UserID:       userID,
		Content:      req.Content,
		ImageURLs:    imageURLsJSON,
		SharedPostID: req.SharedPostID,
		GroupID:      req.GroupID,
		Privacy:      privacy,
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

	// Create tags if provided
	if len(req.Tags) > 0 {
		// Note: In a real implementation, you'd create PostTag records here
		// For now, we'll skip this as it requires additional repository methods
		// TODO: Implement PostTag creation
	}

	// Create location if provided
	if req.Location != nil {
		// Note: In a real implementation, you'd create PostLocation record here
		// For now, we'll skip this as it requires additional repository methods
		// TODO: Implement PostLocation creation
	}

	// Reload with relationships
	return s.postRepo.FindByID(post.ID)
}

// GetPostByID retrieves a post by ID
func (s *postService) GetPostByID(postID string, viewerID string) (*model.Post, error) {
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	// Check if viewer can view this post
	if !s.canViewPost(post, viewerID) {
		return nil, errors.New("unauthorized: you cannot view this post")
	}

	return post, nil
}

// GetPostsByUserID retrieves posts by user ID
func (s *postService) GetPostsByUserID(userID string, viewerID string, limit, offset int) ([]*model.Post, error) {
	// Check if user exists
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	posts, err := s.postRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	// Filter posts based on privacy and viewer relationship
	filteredPosts := []*model.Post{}
	for _, post := range posts {
		if s.canViewPost(post, viewerID) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	return filteredPosts, nil
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

// GetFeed retrieves feed posts for a user
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

// UpdatePost updates a post
func (s *postService) UpdatePost(userID string, postID string, req UpdatePostRequest) (*model.Post, error) {
	// Get existing post
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return nil, errors.New("post not found")
	}

	// Check if user owns this post
	if post.UserID != userID {
		return nil, errors.New("unauthorized: you can only update your own posts")
	}

	// Update fields
	if req.Content != nil {
		post.Content = req.Content
	}
	if req.ImageURLs != nil {
		// Serialize ImageURLs array to JSON string
		if len(req.ImageURLs) > 0 {
			imageURLsBytes, err := json.Marshal(req.ImageURLs)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize image URLs: %w", err)
			}
			post.ImageURLs = string(imageURLsBytes)
		} else {
			post.ImageURLs = ""
		}
	}
	if req.Privacy != nil {
		// Validate privacy
		validPrivacy := map[string]bool{
			model.PrivacyPublic:  true,
			model.PrivacyFriends: true,
			model.PrivacyOnlyMe:  true,
		}
		if !validPrivacy[*req.Privacy] {
			return nil, errors.New("invalid privacy setting")
		}
		post.Privacy = *req.Privacy
	}
	if req.IsPinned != nil {
		post.IsPinned = *req.IsPinned
	}

	if err := s.postRepo.Update(post); err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	// Reload with relationships
	return s.postRepo.FindByID(post.ID)
}

// DeletePost deletes a post
func (s *postService) DeletePost(userID string, postID string) error {
	// Get existing post
	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		return errors.New("post not found")
	}

	// Check if user owns this post
	if post.UserID != userID {
		return errors.New("unauthorized: you can only delete your own posts")
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

// canViewPost checks if a viewer can view a post based on privacy settings
func (s *postService) canViewPost(post *model.Post, viewerID string) bool {
	// Owner can always view their own posts
	if post.UserID == viewerID {
		return true
	}

	// Check privacy settings
	switch post.Privacy {
	case model.PrivacyPublic:
		return true
	case model.PrivacyFriends:
		// Check if viewer is a friend
		friendship, err := s.friendshipRepo.FindBySenderAndReceiver(post.UserID, viewerID)
		if err != nil {
			return false
		}
		return friendship.Status == model.FriendshipStatusAccepted
	case model.PrivacyOnlyMe:
		return false
	default:
		return false
	}
}

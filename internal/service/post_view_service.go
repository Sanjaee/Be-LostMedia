package service

import (
	"errors"
	"fmt"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type PostViewService interface {
	TrackView(userID, postID string) error
	GetViewCount(postID string) (int64, error)
	GetViewsByPostID(postID string, limit, offset int) ([]*model.PostView, error)
	HasUserViewed(userID, postID string) (bool, error)
}

type postViewService struct {
	viewRepo repository.PostViewRepository
	postRepo repository.PostRepository
	userRepo repository.UserRepository
}

func NewPostViewService(
	viewRepo repository.PostViewRepository,
	postRepo repository.PostRepository,
	userRepo repository.UserRepository,
) PostViewService {
	return &postViewService{
		viewRepo: viewRepo,
		postRepo: postRepo,
		userRepo: userRepo,
	}
}

// TrackView tracks a view for a post (prevents duplicates)
func (s *postViewService) TrackView(userID, postID string) error {
	// Validate user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return errors.New("user not found")
	}

	// Validate post exists
	if _, err := s.postRepo.FindByID(postID); err != nil {
		return errors.New("post not found")
	}

	// Check if user already viewed this post
	existingView, err := s.viewRepo.FindByPostAndUser(postID, userID)
	if err == nil && existingView != nil {
		// View already exists, return success (no duplicate)
		return nil
	}

	// Create new view
	view := &model.PostView{
		PostID: postID,
		UserID: userID,
	}

	if err := s.viewRepo.CreateOrUpdate(view); err != nil {
		return fmt.Errorf("failed to track view: %w", err)
	}

	// Update engagement score in Redis (only if new view, not duplicate).
	// Repository applies newness boost so new posts appear at top (post baru muncul paling atas).
	if existingView == nil {
		s.postRepo.UpdatePostEngagementScore(postID)
	}

	return nil
}

// GetViewCount gets the view count for a post
func (s *postViewService) GetViewCount(postID string) (int64, error) {
	return s.viewRepo.CountByPostID(postID)
}

// GetViewsByPostID gets views for a post
func (s *postViewService) GetViewsByPostID(postID string, limit, offset int) ([]*model.PostView, error) {
	return s.viewRepo.FindByPostID(postID)
}

// HasUserViewed checks if a user has viewed a post
func (s *postViewService) HasUserViewed(userID, postID string) (bool, error) {
	_, err := s.viewRepo.FindByPostAndUser(postID, userID)
	if err != nil {
		return false, nil // Not viewed
	}
	return true, nil
}

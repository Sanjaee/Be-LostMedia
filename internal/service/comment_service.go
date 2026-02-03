package service

import (
	"errors"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type CommentService interface {
	CreateComment(userID string, req CreateCommentRequest) (*model.Comment, error)
	GetCommentByID(commentID string) (*model.Comment, error)
	GetCommentsByPostID(postID string, limit, offset int) ([]*model.Comment, int64, error)
	GetRepliesByCommentID(commentID string, limit, offset int) ([]*model.Comment, int64, error)
	UpdateComment(userID, commentID string, req UpdateCommentRequest) (*model.Comment, error)
	DeleteComment(userID, commentID string) error
	GetCommentCount(postID string) (int64, error)
}

type commentService struct {
	commentRepo repository.CommentRepository
	userRepo    repository.UserRepository
	postRepo    repository.PostRepository
}

type CreateCommentRequest struct {
	PostID   string  `json:"post_id" binding:"required"`
	ParentID *string `json:"parent_id,omitempty"` // For replies
	Content  string  `json:"content" binding:"required"`
	MediaURL *string `json:"media_url,omitempty"`
}

type UpdateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

func NewCommentService(
	commentRepo repository.CommentRepository,
	userRepo repository.UserRepository,
	postRepo repository.PostRepository,
) CommentService {
	return &commentService{
		commentRepo: commentRepo,
		userRepo:    userRepo,
		postRepo:    postRepo,
	}
}

// CreateComment creates a new comment
func (s *commentService) CreateComment(userID string, req CreateCommentRequest) (*model.Comment, error) {
	// Validate user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	// Validate post exists
	if _, err := s.postRepo.FindByID(req.PostID); err != nil {
		return nil, errors.New("post not found")
	}

	// If parent_id is provided, validate parent comment exists and belongs to same post
	if req.ParentID != nil && *req.ParentID != "" {
		parent, err := s.commentRepo.FindByID(*req.ParentID)
		if err != nil {
			return nil, errors.New("parent comment not found")
		}
		if parent.PostID != req.PostID {
			return nil, errors.New("parent comment does not belong to this post")
		}
	}

	// Create comment
	comment := &model.Comment{
		PostID:   req.PostID,
		UserID:   userID,
		ParentID: req.ParentID,
		Content:  req.Content,
		MediaURL: req.MediaURL,
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return nil, errors.New("failed to create comment")
	}

	// Reload with relationships
	return s.commentRepo.FindByID(comment.ID)
}

// GetCommentByID gets a comment by ID
func (s *commentService) GetCommentByID(commentID string) (*model.Comment, error) {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return nil, errors.New("comment not found")
	}
	return comment, nil
}

// GetCommentsByPostID gets comments for a post
func (s *commentService) GetCommentsByPostID(postID string, limit, offset int) ([]*model.Comment, int64, error) {
	// Validate post exists
	if _, err := s.postRepo.FindByID(postID); err != nil {
		return nil, 0, errors.New("post not found")
	}

	// Get comments
	comments, err := s.commentRepo.FindByPostID(postID, limit, offset)
	if err != nil {
		return nil, 0, errors.New("failed to get comments")
	}

	// Get total count
	total, err := s.commentRepo.CountByPostID(postID)
	if err != nil {
		return nil, 0, errors.New("failed to get comment count")
	}

	return comments, total, nil
}

// GetRepliesByCommentID gets replies to a comment
func (s *commentService) GetRepliesByCommentID(commentID string, limit, offset int) ([]*model.Comment, int64, error) {
	// Validate comment exists
	if _, err := s.commentRepo.FindByID(commentID); err != nil {
		return nil, 0, errors.New("comment not found")
	}

	// Get replies
	replies, err := s.commentRepo.FindByParentID(commentID, limit, offset)
	if err != nil {
		return nil, 0, errors.New("failed to get replies")
	}

	// Get total count
	total, err := s.commentRepo.CountByParentID(commentID)
	if err != nil {
		return nil, 0, errors.New("failed to get reply count")
	}

	return replies, total, nil
}

// UpdateComment updates a comment
func (s *commentService) UpdateComment(userID, commentID string, req UpdateCommentRequest) (*model.Comment, error) {
	// Get existing comment
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return nil, errors.New("comment not found")
	}

	// Check if user owns this comment
	if comment.UserID != userID {
		return nil, errors.New("unauthorized: you can only update your own comments")
	}

	// Update content
	comment.Content = req.Content

	if err := s.commentRepo.Update(comment); err != nil {
		return nil, errors.New("failed to update comment")
	}

	// Reload with relationships
	return s.commentRepo.FindByID(comment.ID)
}

// DeleteComment deletes a comment
func (s *commentService) DeleteComment(userID, commentID string) error {
	// Get existing comment
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return errors.New("comment not found")
	}

	// Check if user owns this comment
	if comment.UserID != userID {
		return errors.New("unauthorized: you can only delete your own comments")
	}

	if err := s.commentRepo.Delete(commentID); err != nil {
		return errors.New("failed to delete comment")
	}

	return nil
}

// GetCommentCount gets the comment count for a post
func (s *commentService) GetCommentCount(postID string) (int64, error) {
	return s.commentRepo.CountByPostID(postID)
}

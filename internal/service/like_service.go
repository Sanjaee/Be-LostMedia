package service

import (
	"errors"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type LikeService interface {
	LikePost(userID, postID string, reaction string) (*model.Like, error)
	LikeComment(userID, commentID string, reaction string) (*model.Like, error)
	UnlikePost(userID, postID string) error
	UnlikeComment(userID, commentID string) error
	GetLikesByTarget(targetType, targetID string, limit, offset int) ([]*model.Like, int64, error)
	GetLikeCount(targetType, targetID string) (int64, error)
	CheckUserLiked(userID, targetType, targetID string) (bool, *model.Like, error)
}

type likeService struct {
	likeRepo   repository.LikeRepository
	userRepo   repository.UserRepository
	postRepo   repository.PostRepository
	commentRepo repository.CommentRepository
}

func NewLikeService(
	likeRepo repository.LikeRepository,
	userRepo repository.UserRepository,
	postRepo repository.PostRepository,
	commentRepo repository.CommentRepository,
) LikeService {
	return &likeService{
		likeRepo:    likeRepo,
		userRepo:    userRepo,
		postRepo:    postRepo,
		commentRepo: commentRepo,
	}
}

// LikePost likes a post
func (s *likeService) LikePost(userID, postID string, reaction string) (*model.Like, error) {
	// Validate user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	// Validate post exists
	if _, err := s.postRepo.FindByID(postID); err != nil {
		return nil, errors.New("post not found")
	}

	// Validate reaction
	if reaction == "" {
		reaction = model.ReactionLike
	}
	if !isValidReaction(reaction) {
		return nil, errors.New("invalid reaction type")
	}

	// Check if user already liked this post
	existing, err := s.likeRepo.FindByUserAndTarget(userID, model.TargetTypePost, postID)
	if err == nil && existing != nil {
		// Update reaction if different
		if existing.Reaction != reaction {
			existing.Reaction = reaction
			if err := s.likeRepo.Update(existing); err != nil {
				return nil, errors.New("failed to update reaction")
			}
		}
		return existing, nil
	}

	// Create new like
	like := &model.Like{
		UserID:     userID,
		TargetType: model.TargetTypePost,
		TargetID:   postID,
		Reaction:   reaction,
	}

	if err := s.likeRepo.Create(like); err != nil {
		return nil, errors.New("failed to like post")
	}

	return like, nil
}

// LikeComment likes a comment
func (s *likeService) LikeComment(userID, commentID string, reaction string) (*model.Like, error) {
	// Validate user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	// Validate comment exists
	if _, err := s.commentRepo.FindByID(commentID); err != nil {
		return nil, errors.New("comment not found")
	}

	// Validate reaction
	if reaction == "" {
		reaction = model.ReactionLike
	}
	if !isValidReaction(reaction) {
		return nil, errors.New("invalid reaction type")
	}

	// Check if user already liked this comment
	existing, err := s.likeRepo.FindByUserAndTarget(userID, model.TargetTypeComment, commentID)
	if err == nil && existing != nil {
		// Update reaction if different
		if existing.Reaction != reaction {
			existing.Reaction = reaction
			if err := s.likeRepo.Update(existing); err != nil {
				return nil, errors.New("failed to update reaction")
			}
		}
		return existing, nil
	}

	// Create new like
	like := &model.Like{
		UserID:     userID,
		TargetType: model.TargetTypeComment,
		TargetID:   commentID,
		Reaction:   reaction,
	}

	if err := s.likeRepo.Create(like); err != nil {
		return nil, errors.New("failed to like comment")
	}

	return like, nil
}

// UnlikePost unlikes a post
func (s *likeService) UnlikePost(userID, postID string) error {
	// Check if like exists
	like, err := s.likeRepo.FindByUserAndTarget(userID, model.TargetTypePost, postID)
	if err != nil {
		return errors.New("like not found")
	}

	// Delete the like
	if err := s.likeRepo.Delete(like.ID); err != nil {
		return errors.New("failed to unlike post")
	}

	return nil
}

// UnlikeComment unlikes a comment
func (s *likeService) UnlikeComment(userID, commentID string) error {
	// Check if like exists
	like, err := s.likeRepo.FindByUserAndTarget(userID, model.TargetTypeComment, commentID)
	if err != nil {
		return errors.New("like not found")
	}

	// Delete the like
	if err := s.likeRepo.Delete(like.ID); err != nil {
		return errors.New("failed to unlike comment")
	}

	return nil
}

// GetLikesByTarget gets likes for a target (post or comment)
func (s *likeService) GetLikesByTarget(targetType, targetID string, limit, offset int) ([]*model.Like, int64, error) {
	// Validate target type
	if targetType != model.TargetTypePost && targetType != model.TargetTypeComment {
		return nil, 0, errors.New("invalid target type")
	}

	// Get likes
	likes, err := s.likeRepo.FindByTarget(targetType, targetID)
	if err != nil {
		return nil, 0, errors.New("failed to get likes")
	}

	// Get total count
	total, err := s.likeRepo.CountByTarget(targetType, targetID)
	if err != nil {
		return nil, 0, errors.New("failed to get like count")
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(likes) {
		return []*model.Like{}, total, nil
	}
	if end > len(likes) {
		end = len(likes)
	}

	return likes[start:end], total, nil
}

// GetLikeCount gets the like count for a target
func (s *likeService) GetLikeCount(targetType, targetID string) (int64, error) {
	return s.likeRepo.CountByTarget(targetType, targetID)
}

// CheckUserLiked checks if user has liked a target
func (s *likeService) CheckUserLiked(userID, targetType, targetID string) (bool, *model.Like, error) {
	like, err := s.likeRepo.FindByUserAndTarget(userID, targetType, targetID)
	if err != nil {
		return false, nil, nil // Not liked, but no error
	}
	return true, like, nil
}

// isValidReaction validates reaction type
func isValidReaction(reaction string) bool {
	validReactions := []string{
		model.ReactionLike,
		model.ReactionLove,
		model.ReactionHaha,
		model.ReactionWow,
		model.ReactionSad,
		model.ReactionAngry,
	}

	for _, valid := range validReactions {
		if reaction == valid {
			return true
		}
	}
	return false
}

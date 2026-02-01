package service

import (
	"errors"
	"fmt"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type FriendshipService interface {
	SendFriendRequest(senderID, receiverID string) (*model.Friendship, error)
	AcceptFriendRequest(friendshipID, userID string) (*model.Friendship, error)
	RejectFriendRequest(friendshipID, userID string) error
	RemoveFriend(friendshipID, userID string) error
	GetFriendshipByID(friendshipID string) (*model.Friendship, error)
	GetFriendshipsByUserID(userID string) ([]*model.Friendship, error)
	GetPendingRequests(userID string) ([]*model.Friendship, error)
	GetFriends(userID string) ([]*model.Friendship, error)
	GetFriendshipStatus(userID1, userID2 string) (string, error)
}

type friendshipService struct {
	friendshipRepo repository.FriendshipRepository
	userRepo       repository.UserRepository
	notifService   NotificationService
}

func NewFriendshipService(
	friendshipRepo repository.FriendshipRepository,
	userRepo repository.UserRepository,
	notifService NotificationService,
) FriendshipService {
	return &friendshipService{
		friendshipRepo: friendshipRepo,
		userRepo:       userRepo,
		notifService:   notifService,
	}
}

// SendFriendRequest sends a friend request
func (s *friendshipService) SendFriendRequest(senderID, receiverID string) (*model.Friendship, error) {
	// Validate users
	if senderID == receiverID {
		return nil, errors.New("cannot send friend request to yourself")
	}

	// Check if sender exists
	sender, err := s.userRepo.FindByID(senderID)
	if err != nil {
		return nil, errors.New("sender not found")
	}

	// Check if receiver exists
	_, err = s.userRepo.FindByID(receiverID)
	if err != nil {
		return nil, errors.New("receiver not found")
	}

	// Check if friendship already exists
	existing, err := s.friendshipRepo.FindBySenderAndReceiver(senderID, receiverID)
	if err == nil && existing != nil {
		switch existing.Status {
		case model.FriendshipStatusPending:
			return nil, errors.New("friend request already pending")
		case model.FriendshipStatusAccepted:
			return nil, errors.New("already friends")
		case model.FriendshipStatusBlocked:
			return nil, errors.New("cannot send request to blocked user")
		}
	}

	// Create friendship request
	friendship := &model.Friendship{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Status:     model.FriendshipStatusPending,
	}

	if err := s.friendshipRepo.Create(friendship); err != nil {
		return nil, fmt.Errorf("failed to create friendship request: %w", err)
	}

	// Send notification via RabbitMQ (async, non-blocking)
	go func() {
		s.notifService.SendFriendRequestNotification(
			receiverID,
			senderID,
			sender.FullName,
			friendship.ID,
		)
	}()

	// Reload with relationships
	return s.friendshipRepo.FindByID(friendship.ID)
}

// AcceptFriendRequest accepts a friend request
func (s *friendshipService) AcceptFriendRequest(friendshipID, userID string) (*model.Friendship, error) {
	// Get friendship
	friendship, err := s.friendshipRepo.FindByID(friendshipID)
	if err != nil {
		return nil, errors.New("friendship not found")
	}

	// Validate user is the receiver
	if friendship.ReceiverID != userID {
		return nil, errors.New("unauthorized: you can only accept requests sent to you")
	}

	// Check if already accepted
	if friendship.Status == model.FriendshipStatusAccepted {
		return friendship, nil
	}

	// Check if request is pending
	if friendship.Status != model.FriendshipStatusPending {
		return nil, errors.New("cannot accept a non-pending request")
	}

	// Update status
	friendship.Status = model.FriendshipStatusAccepted
	if err := s.friendshipRepo.Update(friendship); err != nil {
		return nil, fmt.Errorf("failed to accept friendship: %w", err)
	}

	// Send notification to sender (async)
	go func() {
		receiver, _ := s.userRepo.FindByID(friendship.ReceiverID)
		if receiver != nil {
			s.notifService.SendFriendAcceptedNotification(
				friendship.SenderID,
				friendship.ReceiverID,
				receiver.FullName,
				friendship.ID,
			)
		}
	}()

	// Reload with relationships
	return s.friendshipRepo.FindByID(friendship.ID)
}

// RejectFriendRequest rejects a friend request
func (s *friendshipService) RejectFriendRequest(friendshipID, userID string) error {
	// Get friendship
	friendship, err := s.friendshipRepo.FindByID(friendshipID)
	if err != nil {
		return errors.New("friendship not found")
	}

	// Validate user is the receiver
	if friendship.ReceiverID != userID {
		return errors.New("unauthorized: you can only reject requests sent to you")
	}

	// Check if request is pending
	if friendship.Status != model.FriendshipStatusPending {
		return errors.New("cannot reject a non-pending request")
	}

	// Update status
	friendship.Status = model.FriendshipStatusRejected
	if err := s.friendshipRepo.Update(friendship); err != nil {
		return fmt.Errorf("failed to reject friendship: %w", err)
	}

	// Send notification to sender (async)
	go func() {
		receiver, _ := s.userRepo.FindByID(friendship.ReceiverID)
		if receiver != nil {
			s.notifService.SendFriendRejectedNotification(
				friendship.SenderID,
				friendship.ReceiverID,
				receiver.FullName,
				friendship.ID,
			)
		}
	}()

	return nil
}

// RemoveFriend removes a friendship
func (s *friendshipService) RemoveFriend(friendshipID, userID string) error {
	// Get friendship
	friendship, err := s.friendshipRepo.FindByID(friendshipID)
	if err != nil {
		return errors.New("friendship not found")
	}

	// Validate user is part of the friendship
	if friendship.SenderID != userID && friendship.ReceiverID != userID {
		return errors.New("unauthorized: you can only remove your own friendships")
	}

	// Delete friendship
	if err := s.friendshipRepo.Delete(friendshipID); err != nil {
		return fmt.Errorf("failed to remove friendship: %w", err)
	}

	return nil
}

// GetFriendshipByID gets a friendship by ID
func (s *friendshipService) GetFriendshipByID(friendshipID string) (*model.Friendship, error) {
	friendship, err := s.friendshipRepo.FindByID(friendshipID)
	if err != nil {
		return nil, errors.New("friendship not found")
	}
	return friendship, nil
}

// GetFriendshipsByUserID gets all friendships for a user
func (s *friendshipService) GetFriendshipsByUserID(userID string) ([]*model.Friendship, error) {
	return s.friendshipRepo.FindByUserID(userID)
}

// GetPendingRequests gets pending friend requests for a user
func (s *friendshipService) GetPendingRequests(userID string) ([]*model.Friendship, error) {
	return s.friendshipRepo.FindPendingByReceiverID(userID)
}

// GetFriends gets accepted friends for a user
func (s *friendshipService) GetFriends(userID string) ([]*model.Friendship, error) {
	return s.friendshipRepo.FindAcceptedByUserID(userID)
}

// GetFriendshipStatus gets the friendship status between two users
func (s *friendshipService) GetFriendshipStatus(userID1, userID2 string) (string, error) {
	friendship, err := s.friendshipRepo.FindBySenderAndReceiver(userID1, userID2)
	if err != nil {
		return "none", nil // No friendship exists
	}
	return friendship.Status, nil
}

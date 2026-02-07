package service

import (
	"errors"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type ChatService interface {
	SendMessage(senderID, receiverID, content string) (*model.ChatMessage, error)
	GetConversation(userID, otherUserID string, limit, offset int) ([]*model.ChatMessage, error)
	MarkAsRead(userID, senderID string) error
	GetUnreadCount(userID string) (int64, error)
	GetUnreadCountBySenders(userID string) (map[string]int64, error)
}

type chatService struct {
	chatRepo   repository.ChatRepository
	userRepo   repository.UserRepository
	friendRepo repository.FriendshipRepository
}

func NewChatService(
	chatRepo repository.ChatRepository,
	userRepo repository.UserRepository,
	friendRepo repository.FriendshipRepository,
) ChatService {
	return &chatService{
		chatRepo:   chatRepo,
		userRepo:   userRepo,
		friendRepo: friendRepo,
	}
}

func (s *chatService) SendMessage(senderID, receiverID, content string) (*model.ChatMessage, error) {
	if content == "" {
		return nil, errors.New("message content cannot be empty")
	}
	if senderID == receiverID {
		return nil, errors.New("cannot send message to yourself")
	}
	if _, err := s.userRepo.FindByID(receiverID); err != nil {
		return nil, errors.New("receiver not found")
	}
	// Check friendship - only friends can chat
	friendship, fErr := s.friendRepo.FindBySenderAndReceiver(senderID, receiverID)
	if fErr != nil || friendship.Status != "accepted" {
		return nil, errors.New("can only chat with friends")
	}

	msg := &model.ChatMessage{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
	}
	if err := s.chatRepo.Create(msg); err != nil {
		return nil, err
	}
	return s.chatRepo.FindByID(msg.ID)
}

func (s *chatService) GetConversation(userID, otherUserID string, limit, offset int) ([]*model.ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.chatRepo.GetConversation(userID, otherUserID, limit, offset)
}

func (s *chatService) MarkAsRead(userID, senderID string) error {
	return s.chatRepo.MarkAsRead(userID, senderID)
}

func (s *chatService) GetUnreadCount(userID string) (int64, error) {
	return s.chatRepo.GetUnreadCount(userID)
}

func (s *chatService) GetUnreadCountBySenders(userID string) (map[string]int64, error) {
	return s.chatRepo.GetUnreadCountBySenders(userID)
}

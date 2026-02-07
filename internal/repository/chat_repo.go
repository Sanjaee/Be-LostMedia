package repository

import (
	"yourapp/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	Create(msg *model.ChatMessage) error
	FindByID(id string) (*model.ChatMessage, error)
	GetConversation(senderID, receiverID string, limit, offset int) ([]*model.ChatMessage, error)
	MarkAsRead(receiverID, senderID string) error
	GetUnreadCount(userID string) (int64, error)
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) Create(msg *model.ChatMessage) error {
	return r.db.Create(msg).Error
}

func (r *chatRepository) FindByID(id string) (*model.ChatMessage, error) {
	var msg model.ChatMessage
	err := r.db.Preload("Sender").Preload("Receiver").Where("id = ?", id).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *chatRepository) GetConversation(senderID, receiverID string, limit, offset int) ([]*model.ChatMessage, error) {
	var messages []*model.ChatMessage
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			senderID, receiverID, receiverID, senderID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (r *chatRepository) MarkAsRead(receiverID, senderID string) error {
	return r.db.Model(&model.ChatMessage{}).
		Where("receiver_id = ? AND sender_id = ? AND is_read = ?", receiverID, senderID, false).
		Update("is_read", true).Error
}

func (r *chatRepository) GetUnreadCount(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ChatMessage{}).
		Where("receiver_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/repository"
	"yourapp/internal/util"
)

type NotificationService interface {
	SendFriendRequestNotification(receiverID, senderID, senderName, friendshipID string) error
	SendFriendAcceptedNotification(receiverID, senderID, senderName, friendshipID string) error
	SendFriendRejectedNotification(receiverID, senderID, senderName, friendshipID string) error
	SendFriendRemovedNotification(receiverID, senderID, senderName string) error
	GetNotificationsByUserID(userID string, limit, offset int) ([]*model.Notification, error)
	GetUnreadNotifications(userID string) ([]*model.Notification, error)
	GetUnreadCount(userID string) (int64, error)
	MarkAsRead(notificationID, userID string) error
	MarkAllAsRead(userID string) error
	DeleteNotification(notificationID, userID string) error
	DeleteByTargetIDAndType(targetID, notifType string) error
	SetWSHub(hub interface {
		BroadcastToUser(string, map[string]interface{})
	})
}

type notificationService struct {
	notifRepo repository.NotificationRepository
	rabbitMQ  *util.RabbitMQClient
	wsHub     interface {
		BroadcastToUser(string, map[string]interface{})
	} // WebSocket hub interface
}

// NotificationMessage represents the message structure for RabbitMQ
type NotificationMessage struct {
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

const (
	NotificationQueueName = "notification_queue"
	NotificationExchange  = "notification_exchange"
)

func NewNotificationService(notifRepo repository.NotificationRepository, rabbitMQ *util.RabbitMQClient) NotificationService {
	return &notificationService{
		notifRepo: notifRepo,
		rabbitMQ:  rabbitMQ,
		wsHub:     nil, // Will be set via SetWSHub
	}
}

// SetWSHub sets the WebSocket hub for realtime notifications
func (s *notificationService) SetWSHub(hub interface {
	BroadcastToUser(string, map[string]interface{})
}) {
	s.wsHub = hub
}

// sendNotification sends notification via RabbitMQ and saves to DB
func (s *notificationService) sendNotification(
	userID, notifType, title, message string,
	data map[string]interface{},
) error {
	// Create notification in database
	notification := &model.Notification{
		UserID:  userID,
		Type:    notifType,
		Title:   title,
		Message: message,
		IsRead:  false,
	}

	// Extract sender_id and target_id from data if available
	if data != nil {
		if senderID, ok := data["sender_id"].(string); ok {
			notification.SenderID = &senderID
		}
		if targetID, ok := data["friendship_id"].(string); ok {
			notification.TargetID = &targetID
		} else if targetID, ok := data["target_id"].(string); ok {
			notification.TargetID = &targetID
		}

		// Convert data to JSON string if provided
		dataJSON, err := json.Marshal(data)
		if err == nil {
			notification.Data = string(dataJSON)
		}
	}

	if err := s.notifRepo.Create(notification); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Send to RabbitMQ for async processing and WebSocket push
	if s.rabbitMQ != nil {
		msg := NotificationMessage{
			UserID:    userID,
			Type:      notifType,
			Title:     title,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Failed to marshal notification message: %v", err)
			return err
		}

		// Publish to RabbitMQ (async, non-blocking)
		if s.rabbitMQ != nil {
			if err := s.rabbitMQ.Publish(NotificationExchange, NotificationQueueName, msgJSON); err != nil {
				log.Printf("Failed to publish notification to RabbitMQ: %v", err)
				// Don't return error, notification is already saved in DB
			}
		}
	}

	// Push to WebSocket if hub is available
	if s.wsHub != nil {
		// Prepare notification payload for WebSocket
		// Format: direct notification object (not wrapped in payload)
		wsPayload := map[string]interface{}{
			"id":         notification.ID,
			"user_id":    notification.UserID,
			"type":       notification.Type,
			"title":      notification.Title,
			"message":    notification.Message,
			"target_id":  notification.TargetID,
			"is_read":    notification.IsRead,
			"created_at": notification.CreatedAt.Format(time.RFC3339),
		}

		// Add sender_id if available
		if notification.SenderID != nil {
			wsPayload["sender_id"] = *notification.SenderID
		}

		// Add data if available
		if notification.Data != "" {
			var dataMap map[string]interface{}
			if err := json.Unmarshal([]byte(notification.Data), &dataMap); err == nil {
				wsPayload["data"] = dataMap
			}
		}

		// Broadcast to user via WebSocket
		s.wsHub.BroadcastToUser(userID, wsPayload)
	}

	return nil
}

// SendFriendRequestNotification sends a friend request notification
func (s *notificationService) SendFriendRequestNotification(
	receiverID, senderID, senderName, friendshipID string,
) error {
	title := "New Friend Request"
	message := fmt.Sprintf("%s sent you a friend request", senderName)
	data := map[string]interface{}{
		"friendship_id": friendshipID,
		"sender_id":     senderID,
		"sender_name":   senderName,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypeFriendRequest,
		title,
		message,
		data,
	)
}

// SendFriendAcceptedNotification sends a friend accepted notification
func (s *notificationService) SendFriendAcceptedNotification(
	receiverID, senderID, senderName, friendshipID string,
) error {
	title := "Friend Request Accepted"
	message := fmt.Sprintf("%s accepted your friend request", senderName)
	data := map[string]interface{}{
		"friendship_id": friendshipID,
		"sender_id":     senderID,
		"sender_name":   senderName,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypeFriendAccepted,
		title,
		message,
		data,
	)
}

// SendFriendRejectedNotification sends a friend rejected notification
func (s *notificationService) SendFriendRejectedNotification(
	receiverID, senderID, senderName, friendshipID string,
) error {
	title := "Friend Request Rejected"
	message := fmt.Sprintf("%s rejected your friend request", senderName)
	data := map[string]interface{}{
		"friendship_id": friendshipID,
		"sender_id":     senderID,
		"sender_name":   senderName,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypeFriendRejected,
		title,
		message,
		data,
	)
}

// SendFriendRemovedNotification sends a friend removed notification
func (s *notificationService) SendFriendRemovedNotification(
	receiverID, senderID, senderName string,
) error {
	title := "Friend Removed"
	message := fmt.Sprintf("%s removed you from their friends list", senderName)
	data := map[string]interface{}{
		"sender_id":   senderID,
		"sender_name": senderName,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypeFriendRemoved,
		title,
		message,
		data,
	)
}

// GetNotificationsByUserID gets notifications for a user with pagination
func (s *notificationService) GetNotificationsByUserID(userID string, limit, offset int) ([]*model.Notification, error) {
	return s.notifRepo.FindByUserID(userID, limit, offset)
}

// GetUnreadNotifications gets unread notifications for a user
func (s *notificationService) GetUnreadNotifications(userID string) ([]*model.Notification, error) {
	return s.notifRepo.FindUnreadByUserID(userID)
}

// GetUnreadCount gets unread notification count for a user
func (s *notificationService) GetUnreadCount(userID string) (int64, error) {
	return s.notifRepo.CountUnreadByUserID(userID)
}

// MarkAsRead marks a notification as read
func (s *notificationService) MarkAsRead(notificationID, userID string) error {
	// Verify notification belongs to user
	notification, err := s.notifRepo.FindByID(notificationID)
	if err != nil {
		return errors.New("notification not found")
	}

	if notification.UserID != userID {
		return errors.New("unauthorized: you can only mark your own notifications as read")
	}

	return s.notifRepo.MarkAsRead(notificationID)
}

// MarkAllAsRead marks all notifications as read for a user
func (s *notificationService) MarkAllAsRead(userID string) error {
	return s.notifRepo.MarkAllAsRead(userID)
}

// DeleteNotification deletes a notification
func (s *notificationService) DeleteNotification(notificationID, userID string) error {
	// Verify notification belongs to user
	notification, err := s.notifRepo.FindByID(notificationID)
	if err != nil {
		return errors.New("notification not found")
	}

	if notification.UserID != userID {
		return errors.New("unauthorized: you can only delete your own notifications")
	}

	return s.notifRepo.Delete(notificationID)
}

// DeleteByTargetIDAndType deletes notifications by target_id and type
func (s *notificationService) DeleteByTargetIDAndType(targetID, notifType string) error {
	return s.notifRepo.DeleteByTargetIDAndType(targetID, notifType)
}

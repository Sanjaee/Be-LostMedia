package service

import (
	"encoding/json"
	"errors"
	"fmt"
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
	SendCommentReplyNotification(receiverID, senderID, senderName, commentID, postID string, commentContent string) error
	SendPostCommentNotification(receiverID, senderID, senderName, commentID, postID string, commentContent string) error
	SendPostUploadCompletedNotification(userID, postID string, mediaCount int, mediaType ...string) error
	SendPostLikedNotification(receiverID, senderID, senderName, postID string) error
	SendRoleUpdatedNotification(receiverID, senderID, senderName, newRole string) error
	SendRolePurchasedNotification(userID, roleName, roleLabel string, orderID string) error
	CheckPostLikedNotificationExists(senderID, postID string) (bool, error)
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
		// For comment_reply and post_comment, use comment_id as target_id
		if notifType == model.NotificationTypeCommentReply || notifType == model.NotificationTypePostComment {
			if commentID, ok := data["comment_id"].(string); ok {
				notification.TargetID = &commentID
			}
		} else if notifType == model.NotificationTypePostUploadCompleted || notifType == model.NotificationTypePostLiked {
			// For post upload completed and post liked, use post_id as target_id
			if postID, ok := data["post_id"].(string); ok {
				notification.TargetID = &postID
			}
		} else {
			// For other types, use friendship_id or target_id
			if targetID, ok := data["friendship_id"].(string); ok {
				notification.TargetID = &targetID
			} else if targetID, ok := data["target_id"].(string); ok {
				notification.TargetID = &targetID
			}
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

	// Push directly to WebSocket (simplified, no RabbitMQ for now)
	// TODO: Re-enable RabbitMQ for async processing later

	// Push to WebSocket if hub is available
	// IMPORTANT: WebSocket only sends notification data, NOT friendship status
	// Frontend must always fetch status from DB via API, never from WebSocket
	if s.wsHub != nil {
		// Prepare notification payload for WebSocket
		// Format: direct notification object (not wrapped in payload)
		// NOTE: This only contains notification info, NOT friendship status
		// Frontend will trigger DB refresh when receiving friendship-related notifications
		wsPayload := map[string]interface{}{
			"id":         notification.ID,
			"user_id":    notification.UserID,
			"type":       notification.Type,
			"title":      notification.Title,
			"message":    notification.Message,
			"is_read":    notification.IsRead,
			"created_at": notification.CreatedAt.Format(time.RFC3339),
		}
		if notification.TargetID != nil {
			wsPayload["target_id"] = *notification.TargetID
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

// SendCommentReplyNotification sends a comment reply notification
func (s *notificationService) SendCommentReplyNotification(
	receiverID, senderID, senderName, commentID, postID, commentContent string,
) error {
	// Truncate comment content if too long
	previewContent := commentContent
	if len(previewContent) > 100 {
		previewContent = previewContent[:100] + "..."
	}

	title := "New Reply to Your Comment"
	message := fmt.Sprintf("%s replied to your comment: %s", senderName, previewContent)
	data := map[string]interface{}{
		"sender_id":       senderID,
		"sender_name":     senderName,
		"comment_id":      commentID,
		"post_id":         postID,
		"comment_content": commentContent,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypeCommentReply,
		title,
		message,
		data,
	)
}

// SendPostCommentNotification sends a post comment notification
func (s *notificationService) SendPostCommentNotification(
	receiverID, senderID, senderName, commentID, postID, commentContent string,
) error {
	// Truncate comment content if too long
	previewContent := commentContent
	if len(previewContent) > 100 {
		previewContent = previewContent[:100] + "..."
	}

	title := "New Comment on Your Post"
	message := fmt.Sprintf("%s commented on your post: %s", senderName, previewContent)
	data := map[string]interface{}{
		"sender_id":       senderID,
		"sender_name":     senderName,
		"comment_id":      commentID,
		"post_id":         postID,
		"comment_content": commentContent,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypePostComment,
		title,
		message,
		data,
	)
}

// SendPostUploadCompletedNotification sends a post upload completed notification
// mediaType is optional; defaults to "gambar" (image). Pass "video" for video uploads.
func (s *notificationService) SendPostUploadCompletedNotification(userID, postID string, mediaCount int, mediaType ...string) error {
	mt := "gambar"
	if len(mediaType) > 0 && mediaType[0] != "" {
		mt = mediaType[0]
	}
	title := "Upload Selesai"
	message := fmt.Sprintf("Post berhasil diupload dengan %d %s", mediaCount, mt)
	data := map[string]interface{}{
		"post_id":     postID,
		"media_count": mediaCount,
		"media_type":  mt,
	}

	return s.sendNotification(
		userID,
		model.NotificationTypePostUploadCompleted,
		title,
		message,
		data,
	)
}

// CheckPostLikedNotificationExists checks if a post liked notification already exists for this sender and post
func (s *notificationService) CheckPostLikedNotificationExists(senderID, postID string) (bool, error) {
	// Check if notification with type "post_liked" exists for this post and sender
	// We'll check by looking for notifications with target_id = postID and sender_id = senderID
	notifications, err := s.notifRepo.FindByUserID(senderID, 100, 0) // Get recent notifications
	if err != nil {
		return false, err
	}

	// Check if any notification matches: type = post_liked, target_id = postID, sender_id = senderID
	for _, notif := range notifications {
		if notif.Type == model.NotificationTypePostLiked &&
			notif.TargetID != nil && *notif.TargetID == postID &&
			notif.SenderID != nil && *notif.SenderID == senderID {
			return true, nil
		}
	}

	return false, nil
}

// SendPostLikedNotification sends a post liked notification (only once per user per post)
func (s *notificationService) SendPostLikedNotification(receiverID, senderID, senderName, postID string) error {
	// Check if notification already exists by querying receiver's notifications
	// Get recent notifications for the receiver
	notifications, err := s.notifRepo.FindByUserID(receiverID, 1000, 0)
	if err == nil {
		// Check if any notification matches: type = post_liked, target_id = postID, sender_id = senderID
		for _, notif := range notifications {
			if notif.Type == model.NotificationTypePostLiked &&
				notif.TargetID != nil && *notif.TargetID == postID &&
				notif.SenderID != nil && *notif.SenderID == senderID {
				// Notification already exists, don't send again
				return nil
			}
		}
	}

	title := "Post Disukai"
	message := fmt.Sprintf("%s menyukai post Anda", senderName)
	data := map[string]interface{}{
		"post_id":     postID,
		"sender_id":   senderID,
		"sender_name": senderName,
	}

	return s.sendNotification(
		receiverID,
		model.NotificationTypePostLiked,
		title,
		message,
		data,
	)
}

// SendRolePurchasedNotification sends a notification when user successfully purchases/upgrades role
func (s *notificationService) SendRolePurchasedNotification(userID, roleName, roleLabel, orderID string) error {
	title := "Role Berhasil Dibeli"
	message := fmt.Sprintf("Selamat! Role Anda telah di-upgrade menjadi %s.", roleLabel)
	data := map[string]interface{}{
		"role":      roleName,
		"order_id":  orderID,
		"role_name": roleLabel,
	}
	if orderID != "" {
		data["target_id"] = orderID
	}
	return s.sendNotification(userID, model.NotificationTypeRolePurchased, title, message, data)
}

// SendRoleUpdatedNotification sends a notification when owner changes a user's role
func (s *notificationService) SendRoleUpdatedNotification(receiverID, senderID, senderName, newRole string) error {
	roleLabels := map[string]string{
		"owner": "Owner", "admin": "Admin", "mod": "Moderator",
		"mvp": "MVP", "god": "God", "vip": "VIP", "member": "Member",
	}
	roleLabel := roleLabels[newRole]
	if roleLabel == "" {
		roleLabel = newRole
	}
	title := "Role Diubah"
	message := fmt.Sprintf("Role Anda telah diubah menjadi %s oleh owner (%s).", roleLabel, senderName)
	data := map[string]interface{}{
		"sender_id":   senderID,
		"sender_name": senderName,
		"role":        newRole,
	}
	return s.sendNotification(
		receiverID,
		model.NotificationTypeRoleUpdated,
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

package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(notification *model.Notification) error
	FindByID(id string) (*model.Notification, error)
	FindByUserID(userID string, limit, offset int) ([]*model.Notification, error)
	FindUnreadByUserID(userID string) ([]*model.Notification, error)
	CountUnreadByUserID(userID string) (int64, error)
	MarkAsRead(id string) error
	MarkAllAsRead(userID string) error
	Delete(id string) error
	DeleteByUserID(userID string) error
}

type notificationRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	notificationCachePrefix       = "notification:"
	notificationByUserCachePrefix = "notification:user:"
	notificationUnreadCachePrefix = "notification:unread:"
	notificationCountCachePrefix  = "notification:count:"
	notificationCacheExpiration   = 10 * time.Minute
)

func NewNotificationRepository(db *gorm.DB, redis *util.RedisClient) NotificationRepository {
	return &notificationRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new notification
func (r *notificationRepository) Create(notification *model.Notification) error {
	if err := r.db.Create(notification).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(notification.UserID)
		r.invalidateUnreadCache(notification.UserID)
		r.invalidateCountCache(notification.UserID)
	}

	return nil
}

// FindByID finds a notification by ID
func (r *notificationRepository) FindByID(id string) (*model.Notification, error) {
	var notification model.Notification
	err := r.db.Preload("User").Where("id = ?", id).First(&notification).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

// FindByUserID finds notifications for a user with pagination
func (r *notificationRepository) FindByUserID(userID string, limit, offset int) ([]*model.Notification, error) {
	var notifications []*model.Notification
	err := r.db.Preload("User").Preload("Sender").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

// FindUnreadByUserID finds unread notifications for a user
func (r *notificationRepository) FindUnreadByUserID(userID string) ([]*model.Notification, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getListFromCache(notificationUnreadCachePrefix + userID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	var notifications []*model.Notification
	err := r.db.Preload("User").Preload("Sender").
		Where("user_id = ? AND is_read = ?", userID, false).
		Order("created_at DESC").
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheNotificationList(notificationUnreadCachePrefix+userID, notifications)
	}

	return notifications, nil
}

// CountUnreadByUserID counts unread notifications for a user
func (r *notificationRepository) CountUnreadByUserID(userID string) (int64, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.redis.Get(notificationCountCachePrefix + userID)
		if err == nil {
			var count int64
			if _, err := fmt.Sscanf(cached, "%d", &count); err == nil {
				return count, nil
			}
		}
	}

	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(notificationCountCachePrefix+userID, fmt.Sprintf("%d", count), notificationCacheExpiration)
	}

	return count, nil
}

// MarkAsRead marks a notification as read
func (r *notificationRepository) MarkAsRead(id string) error {
	// Get notification first for cache invalidation
	var notification model.Notification
	if err := r.db.Where("id = ?", id).First(&notification).Error; err != nil {
		return err
	}

	now := time.Now()
	err := r.db.Model(&model.Notification{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		}).Error
	if err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(notification.UserID)
		r.invalidateUnreadCache(notification.UserID)
		r.invalidateCountCache(notification.UserID)
	}

	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (r *notificationRepository) MarkAllAsRead(userID string) error {
	now := time.Now()
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		}).Error
	if err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(userID)
		r.invalidateUnreadCache(userID)
		r.invalidateCountCache(userID)
	}

	return nil
}

// Delete deletes a notification
func (r *notificationRepository) Delete(id string) error {
	// Get notification first for cache invalidation
	var notification model.Notification
	if err := r.db.Where("id = ?", id).First(&notification).Error; err != nil {
		return err
	}

	userID := notification.UserID

	// Delete from database
	if err := r.db.Delete(&notification).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(userID)
		r.invalidateUnreadCache(userID)
		r.invalidateCountCache(userID)
	}

	return nil
}

// DeleteByUserID deletes all notifications for a user
func (r *notificationRepository) DeleteByUserID(userID string) error {
	if err := r.db.Where("user_id = ?", userID).Delete(&model.Notification{}).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(userID)
		r.invalidateUnreadCache(userID)
		r.invalidateCountCache(userID)
	}

	return nil
}

// Cache helpers
func (r *notificationRepository) cacheNotificationList(key string, notifications []*model.Notification) {
	if r.redis == nil {
		return
	}

	notificationsJSON, err := json.Marshal(notifications)
	if err != nil {
		return
	}

	r.redis.Set(key, string(notificationsJSON), notificationCacheExpiration)
}

func (r *notificationRepository) getListFromCache(key string) ([]*model.Notification, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var notifications []*model.Notification
	if err := json.Unmarshal([]byte(cached), &notifications); err != nil {
		return nil, err
	}

	return notifications, nil
}

func (r *notificationRepository) invalidateUserCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.DeletePattern(notificationByUserCachePrefix + userID + ":*")
}

func (r *notificationRepository) invalidateUnreadCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(notificationUnreadCachePrefix + userID)
}

func (r *notificationRepository) invalidateCountCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(notificationCountCachePrefix + userID)
}

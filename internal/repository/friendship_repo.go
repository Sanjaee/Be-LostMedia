package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type FriendshipRepository interface {
	Create(friendship *model.Friendship) error
	FindByID(id string) (*model.Friendship, error)
	FindBySenderAndReceiver(senderID, receiverID string) (*model.Friendship, error)
	FindByUserID(userID string) ([]*model.Friendship, error)
	FindPendingByReceiverID(receiverID string) ([]*model.Friendship, error)
	FindAcceptedByUserID(userID string) ([]*model.Friendship, error)
	Update(friendship *model.Friendship) error
	Delete(id string) error
	DeleteBySenderAndReceiver(senderID, receiverID string) error
	CountPendingByReceiverID(receiverID string) (int64, error)
}

type friendshipRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

const (
	friendshipCachePrefix         = "friendship:"
	friendshipByUserCachePrefix   = "friendship:user:"
	friendshipPendingCachePrefix  = "friendship:pending:"
	friendshipAcceptedCachePrefix = "friendship:accepted:"
	friendshipCountCachePrefix    = "friendship:count:"
	friendshipCacheExpiration     = 15 * time.Minute
)

func NewFriendshipRepository(db *gorm.DB, redis *util.RedisClient) FriendshipRepository {
	return &friendshipRepository{
		db:    db,
		redis: redis,
	}
}

// Create creates a new friendship request
func (r *friendshipRepository) Create(friendship *model.Friendship) error {
	if err := r.db.Create(friendship).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(friendship.SenderID)
		r.invalidateUserCache(friendship.ReceiverID)
		r.invalidatePendingCache(friendship.ReceiverID)
		r.invalidateCountCache(friendship.ReceiverID)
	}

	return nil
}

// FindByID finds a friendship by ID
func (r *friendshipRepository) FindByID(id string) (*model.Friendship, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getFromCache(friendshipCachePrefix + id)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	var friendship model.Friendship
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("id = ?", id).First(&friendship).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheFriendship(&friendship)
	}

	return &friendship, nil
}

// FindBySenderAndReceiver finds friendship between two users
func (r *friendshipRepository) FindBySenderAndReceiver(senderID, receiverID string) (*model.Friendship, error) {
	var friendship model.Friendship
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			senderID, receiverID, receiverID, senderID).
		First(&friendship).Error
	if err != nil {
		return nil, err
	}
	return &friendship, nil
}

// FindByUserID finds all friendships for a user
func (r *friendshipRepository) FindByUserID(userID string) ([]*model.Friendship, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getListFromCache(friendshipByUserCachePrefix + userID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	var friendships []*model.Friendship
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("sender_id = ? OR receiver_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&friendships).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheFriendshipList(friendshipByUserCachePrefix+userID, friendships)
	}

	return friendships, nil
}

// FindPendingByReceiverID finds pending friendship requests for a user
func (r *friendshipRepository) FindPendingByReceiverID(receiverID string) ([]*model.Friendship, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getListFromCache(friendshipPendingCachePrefix + receiverID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	var friendships []*model.Friendship
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("receiver_id = ? AND status = ?", receiverID, model.FriendshipStatusPending).
		Order("created_at DESC").
		Find(&friendships).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheFriendshipList(friendshipPendingCachePrefix+receiverID, friendships)
	}

	return friendships, nil
}

// FindAcceptedByUserID finds accepted friendships for a user
func (r *friendshipRepository) FindAcceptedByUserID(userID string) ([]*model.Friendship, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.getListFromCache(friendshipAcceptedCachePrefix + userID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	var friendships []*model.Friendship
	err := r.db.Preload("Sender").Preload("Receiver").
		Where("(sender_id = ? OR receiver_id = ?) AND status = ?", userID, userID, model.FriendshipStatusAccepted).
		Order("created_at DESC").
		Find(&friendships).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		r.cacheFriendshipList(friendshipAcceptedCachePrefix+userID, friendships)
	}

	return friendships, nil
}

// Update updates a friendship
func (r *friendshipRepository) Update(friendship *model.Friendship) error {
	if err := r.db.Save(friendship).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateFriendshipCache(friendship.ID)
		r.invalidateUserCache(friendship.SenderID)
		r.invalidateUserCache(friendship.ReceiverID)
		r.invalidatePendingCache(friendship.ReceiverID)
		r.invalidateAcceptedCache(friendship.SenderID)
		r.invalidateAcceptedCache(friendship.ReceiverID)
		r.invalidateCountCache(friendship.ReceiverID)
	}

	return nil
}

// Delete deletes a friendship
func (r *friendshipRepository) Delete(id string) error {
	// Get friendship first for cache invalidation
	var friendship model.Friendship
	if err := r.db.Where("id = ?", id).First(&friendship).Error; err != nil {
		return err
	}

	senderID := friendship.SenderID
	receiverID := friendship.ReceiverID

	// Delete from database
	if err := r.db.Delete(&friendship).Error; err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateFriendshipCache(id)
		r.invalidateUserCache(senderID)
		r.invalidateUserCache(receiverID)
		r.invalidatePendingCache(receiverID)
		r.invalidateAcceptedCache(senderID)
		r.invalidateAcceptedCache(receiverID)
		r.invalidateCountCache(receiverID)
	}

	return nil
}

// DeleteBySenderAndReceiver deletes friendship between two users
func (r *friendshipRepository) DeleteBySenderAndReceiver(senderID, receiverID string) error {
	result := r.db.Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		senderID, receiverID, receiverID, senderID).Delete(&model.Friendship{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("friendship not found")
	}

	// Invalidate cache
	if r.redis != nil {
		r.invalidateUserCache(senderID)
		r.invalidateUserCache(receiverID)
		r.invalidatePendingCache(receiverID)
		r.invalidateAcceptedCache(senderID)
		r.invalidateAcceptedCache(receiverID)
		r.invalidateCountCache(receiverID)
	}

	return nil
}

// CountPendingByReceiverID counts pending requests for a user
func (r *friendshipRepository) CountPendingByReceiverID(receiverID string) (int64, error) {
	// Try cache first
	if r.redis != nil {
		cached, err := r.redis.Get(friendshipCountCachePrefix + receiverID)
		if err == nil {
			var count int64
			if _, err := fmt.Sscanf(cached, "%d", &count); err == nil {
				return count, nil
			}
		}
	}

	var count int64
	err := r.db.Model(&model.Friendship{}).
		Where("receiver_id = ? AND status = ?", receiverID, model.FriendshipStatusPending).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// Cache the count
	if r.redis != nil {
		r.redis.Set(friendshipCountCachePrefix+receiverID, fmt.Sprintf("%d", count), friendshipCacheExpiration)
	}

	return count, nil
}

// Cache helpers
func (r *friendshipRepository) cacheFriendship(friendship *model.Friendship) {
	if r.redis == nil {
		return
	}

	friendshipJSON, err := json.Marshal(friendship)
	if err != nil {
		return
	}

	r.redis.Set(friendshipCachePrefix+friendship.ID, string(friendshipJSON), friendshipCacheExpiration)
}

func (r *friendshipRepository) cacheFriendshipList(key string, friendships []*model.Friendship) {
	if r.redis == nil {
		return
	}

	friendshipsJSON, err := json.Marshal(friendships)
	if err != nil {
		return
	}

	r.redis.Set(key, string(friendshipsJSON), friendshipCacheExpiration)
}

func (r *friendshipRepository) getFromCache(key string) (*model.Friendship, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var friendship model.Friendship
	if err := json.Unmarshal([]byte(cached), &friendship); err != nil {
		return nil, err
	}

	return &friendship, nil
}

func (r *friendshipRepository) getListFromCache(key string) ([]*model.Friendship, error) {
	if r.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	cached, err := r.redis.Get(key)
	if err != nil {
		return nil, err
	}

	var friendships []*model.Friendship
	if err := json.Unmarshal([]byte(cached), &friendships); err != nil {
		return nil, err
	}

	return friendships, nil
}

func (r *friendshipRepository) invalidateFriendshipCache(id string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(friendshipCachePrefix + id)
}

func (r *friendshipRepository) invalidateUserCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(friendshipByUserCachePrefix + userID)
}

func (r *friendshipRepository) invalidatePendingCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(friendshipPendingCachePrefix + userID)
}

func (r *friendshipRepository) invalidateAcceptedCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(friendshipAcceptedCachePrefix + userID)
}

func (r *friendshipRepository) invalidateCountCache(userID string) {
	if r.redis == nil {
		return
	}
	r.redis.Delete(friendshipCountCachePrefix + userID)
}

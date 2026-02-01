package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	notificationService service.NotificationService
	jwtSecret           string
}

func NewNotificationHandler(notificationService service.NotificationService, jwtSecret string) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		jwtSecret:           jwtSecret,
	}
}

// GetNotifications handles getting notifications for current user
// GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	notifications, err := h.notificationService.GetNotificationsByUserID(userID.(string), limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Notifications retrieved successfully", gin.H{
		"notifications": notifications,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetUnreadNotifications handles getting unread notifications
// GET /api/v1/notifications/unread
func (h *NotificationHandler) GetUnreadNotifications(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	notifications, err := h.notificationService.GetUnreadNotifications(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Unread notifications retrieved successfully", gin.H{"notifications": notifications})
}

// GetUnreadCount handles getting unread notification count
// GET /api/v1/notifications/unread/count
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	count, err := h.notificationService.GetUnreadCount(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Unread count retrieved successfully", gin.H{"count": count})
}

// MarkAsRead handles marking a notification as read
// PUT /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		util.BadRequest(c, "Notification ID is required")
		return
	}

	err := h.notificationService.MarkAsRead(notificationID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Notification marked as read", nil)
}

// MarkAllAsRead handles marking all notifications as read
// PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	err := h.notificationService.MarkAllAsRead(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "All notifications marked as read", nil)
}

// DeleteNotification handles deleting a notification
// DELETE /api/v1/notifications/:id
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		util.BadRequest(c, "Notification ID is required")
		return
	}

	err := h.notificationService.DeleteNotification(notificationID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Notification deleted successfully", nil)
}

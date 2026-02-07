package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"yourapp/internal/repository"
	"yourapp/internal/service"
	"yourapp/internal/util"
	"yourapp/internal/websocket"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo           repository.UserRepository
	jwtSecret          string
	wsHub              *websocket.Hub
	notificationService service.NotificationService
}

func NewUserHandler(userRepo repository.UserRepository, jwtSecret string, wsHub *websocket.Hub, notificationService service.NotificationService) *UserHandler {
	return &UserHandler{
		userRepo:            userRepo,
		jwtSecret:           jwtSecret,
		wsHub:               wsHub,
		notificationService: notificationService,
	}
}

// GetAllUsers handles getting all users (owner only)
// GET /api/v1/admin/users
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	users, total, err := h.userRepo.FindAll(limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "Failed to get users", nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Users retrieved successfully", gin.H{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUserStats handles getting user statistics (owner only)
// GET /api/v1/admin/stats
func (h *UserHandler) GetUserStats(c *gin.Context) {
	total, err := h.userRepo.Count()
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "Failed to get user stats", nil)
		return
	}

	// Get users by type
	var ownerCount, memberCount int64
	allUsers, _, err := h.userRepo.FindAll(1000, 0) // Get all to count by type
	if err == nil {
		for _, user := range allUsers {
			if user.UserType == "owner" {
				ownerCount++
			} else {
				memberCount++
			}
		}
	}

	// Get verified vs unverified count
	var verifiedCount, unverifiedCount int64
	for _, user := range allUsers {
		if user.IsVerified {
			verifiedCount++
		} else {
			unverifiedCount++
		}
	}

	util.SuccessResponse(c, http.StatusOK, "User stats retrieved successfully", gin.H{
		"total": total,
		"by_type": gin.H{
			"owner":  ownerCount,
			"member": memberCount,
		},
		"by_verification": gin.H{
			"verified":   verifiedCount,
			"unverified": unverifiedCount,
		},
	})
}

// BanUser handles banning a user (owner only)
// POST /api/v1/admin/users/:id/ban
func (h *UserHandler) BanUser(c *gin.Context) {
	targetID := c.Param("id")
	if targetID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	// Prevent banning yourself
	currentUserID, _ := c.Get("userID")
	if currentUserID.(string) == targetID {
		util.BadRequest(c, "Cannot ban yourself")
		return
	}

	var req struct {
		Duration int    `json:"duration" binding:"required"` // duration in minutes
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	if req.Duration < 1 {
		util.BadRequest(c, "Duration must be at least 1 minute")
		return
	}

	// Check target user exists and is not owner
	targetUser, err := h.userRepo.FindByID(targetID)
	if err != nil || targetUser == nil {
		util.NotFound(c, "User not found")
		return
	}
	if targetUser.UserType == "owner" {
		util.BadRequest(c, "Cannot ban an owner")
		return
	}

	bannedUntil := time.Now().Add(time.Duration(req.Duration) * time.Minute)
	reason := req.Reason
	if reason == "" {
		reason = "Melanggar ketentuan layanan"
	}

	if err := h.userRepo.BanUser(targetID, bannedUntil, reason); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "Failed to ban user", nil)
		return
	}

	// Broadcast ban event to the target user in real-time
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(targetID, map[string]interface{}{
			"type":         "user_banned",
			"banned_until": bannedUntil,
			"ban_reason":   reason,
		})
	}

	util.SuccessResponse(c, http.StatusOK, "User banned successfully", gin.H{
		"user_id":      targetID,
		"banned_until": bannedUntil,
		"reason":       reason,
	})
}

// UnbanUser handles unbanning a user (owner only)
// POST /api/v1/admin/users/:id/unban
func (h *UserHandler) UnbanUser(c *gin.Context) {
	targetID := c.Param("id")
	if targetID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	if err := h.userRepo.UnbanUser(targetID); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "Failed to unban user", nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "User unbanned successfully", gin.H{
		"user_id": targetID,
	})
}

var allowedRoles = map[string]bool{
	"owner": true, "admin": true, "mod": true, "mvp": true, "god": true, "vip": true, "member": true,
}

// UpdateUserRole handles updating a user's role (owner only)
// PUT /api/v1/admin/users/:id/role
func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	targetID := c.Param("id")
	if targetID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "Role is required")
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if !allowedRoles[role] {
		util.BadRequest(c, "Invalid role. Allowed: owner, admin, mod, mvp, god, vip, member")
		return
	}

	currentUserID, _ := c.Get("userID")
	if currentUserID.(string) == targetID {
		util.BadRequest(c, "Cannot change your own role")
		return
	}

	targetUser, err := h.userRepo.FindByID(targetID)
	if err != nil || targetUser == nil {
		util.ErrorResponse(c, http.StatusNotFound, "User not found", nil)
		return
	}

	if err := h.userRepo.UpdateUserRole(targetID, role); err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "Failed to update role", nil)
		return
	}

	// Notify the user (saves to DB + real-time WebSocket) so it appears in NotificationDialog
	ownerID := currentUserID.(string)
	owner, _ := h.userRepo.FindByID(ownerID)
	ownerName := "Owner"
	if owner != nil {
		ownerName = owner.FullName
	}
	if h.notificationService != nil {
		_ = h.notificationService.SendRoleUpdatedNotification(targetID, ownerID, ownerName, role)
	}

	util.SuccessResponse(c, http.StatusOK, "Role updated successfully", gin.H{
		"user_id": targetID,
		"role":    role,
	})
}

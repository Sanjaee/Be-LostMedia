package app

import (
	"net/http"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type FriendshipHandler struct {
	friendshipService service.FriendshipService
	jwtSecret         string
}

func NewFriendshipHandler(friendshipService service.FriendshipService, jwtSecret string) *FriendshipHandler {
	return &FriendshipHandler{
		friendshipService: friendshipService,
		jwtSecret:         jwtSecret,
	}
}

// SendFriendRequest handles sending a friend request
// POST /api/v1/friendships/request
func (h *FriendshipHandler) SendFriendRequest(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req struct {
		ReceiverID string `json:"receiver_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	friendship, err := h.friendshipService.SendFriendRequest(userID.(string), req.ReceiverID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Friend request sent successfully", gin.H{"friendship": friendship})
}

// AcceptFriendRequest handles accepting a friend request
// POST /api/v1/friendships/:id/accept
func (h *FriendshipHandler) AcceptFriendRequest(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendshipID := c.Param("id")
	if friendshipID == "" {
		util.BadRequest(c, "Friendship ID is required")
		return
	}

	friendship, err := h.friendshipService.AcceptFriendRequest(friendshipID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friend request accepted successfully", gin.H{"friendship": friendship})
}

// RejectFriendRequest handles rejecting a friend request
// POST /api/v1/friendships/:id/reject
func (h *FriendshipHandler) RejectFriendRequest(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendshipID := c.Param("id")
	if friendshipID == "" {
		util.BadRequest(c, "Friendship ID is required")
		return
	}

	err := h.friendshipService.RejectFriendRequest(friendshipID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friend request rejected successfully", nil)
}

// RemoveFriend handles removing a friend
// DELETE /api/v1/friendships/:id
func (h *FriendshipHandler) RemoveFriend(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendshipID := c.Param("id")
	if friendshipID == "" {
		util.BadRequest(c, "Friendship ID is required")
		return
	}

	err := h.friendshipService.RemoveFriend(friendshipID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friend removed successfully", nil)
}

// GetFriendship handles getting a friendship by ID
// GET /api/v1/friendships/:id
func (h *FriendshipHandler) GetFriendship(c *gin.Context) {
	friendshipID := c.Param("id")
	if friendshipID == "" {
		util.BadRequest(c, "Friendship ID is required")
		return
	}

	friendship, err := h.friendshipService.GetFriendshipByID(friendshipID)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friendship retrieved successfully", gin.H{"friendship": friendship})
}

// GetMyFriendships handles getting all friendships for current user
// GET /api/v1/friendships
func (h *FriendshipHandler) GetMyFriendships(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendships, err := h.friendshipService.GetFriendshipsByUserID(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friendships retrieved successfully", gin.H{"friendships": friendships})
}

// GetPendingRequests handles getting pending friend requests
// GET /api/v1/friendships/pending
func (h *FriendshipHandler) GetPendingRequests(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendships, err := h.friendshipService.GetPendingRequests(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Pending requests retrieved successfully", gin.H{"friendships": friendships})
}

// GetFriends handles getting accepted friends
// GET /api/v1/friendships/friends
func (h *FriendshipHandler) GetFriends(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	friendships, err := h.friendshipService.GetFriends(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friends retrieved successfully", gin.H{"friends": friendships})
}

// GetFriendshipStatus handles getting friendship status between two users
// GET /api/v1/friendships/status/:userID
func (h *FriendshipHandler) GetFriendshipStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	targetUserID := c.Param("userID")
	if targetUserID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	status, err := h.friendshipService.GetFriendshipStatus(userID.(string), targetUserID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friendship status retrieved successfully", gin.H{"status": status})
}

// GetFriendsCount handles getting friends count for a user
// GET /api/v1/friendships/count/:userID
func (h *FriendshipHandler) GetFriendsCount(c *gin.Context) {
	targetUserID := c.Param("userID")
	if targetUserID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	followers, following, err := h.friendshipService.GetFriendsCount(targetUserID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Friends count retrieved successfully", gin.H{
		"followers": followers,
		"following": following,
	})
}

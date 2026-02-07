package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService service.ChatService
	wsHub       interface {
		BroadcastToUser(string, map[string]interface{})
	}
}

func NewChatHandler(chatService service.ChatService, wsHub interface {
	BroadcastToUser(string, map[string]interface{})
}) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		wsHub:       wsHub,
	}
}

// SendMessageRequest is the request body for sending a chat message
type SendMessageRequest struct {
	ReceiverID string `json:"receiver_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

// SendMessage sends a chat message and pushes to recipient via WebSocket
// POST /api/v1/chat/messages
func (h *ChatHandler) SendMessage(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	msg, err := h.chatService.SendMessage(userID.(string), req.ReceiverID, req.Content)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Push to recipient via WebSocket for real-time delivery
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(req.ReceiverID, map[string]interface{}{
			"type": "chat_message",
			"payload": map[string]interface{}{
				"id":         msg.ID,
				"sender_id":  msg.SenderID,
				"receiver_id": msg.ReceiverID,
				"content":    msg.Content,
				"created_at": msg.CreatedAt,
				"sender":     msg.Sender,
			},
		})
	}

	util.SuccessResponse(c, http.StatusCreated, "Message sent", gin.H{"message": msg})
}

// GetConversation returns messages between current user and another user
// GET /api/v1/chat/messages?with_user_id=xxx&limit=50&offset=0
func (h *ChatHandler) GetConversation(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	withUserID := c.Query("with_user_id")
	if withUserID == "" {
		util.BadRequest(c, "with_user_id is required")
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	messages, err := h.chatService.GetConversation(userID.(string), withUserID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Mark messages from the other user as read
	_ = h.chatService.MarkAsRead(userID.(string), withUserID)

	util.SuccessResponse(c, http.StatusOK, "Conversation retrieved", gin.H{
		"messages": messages,
	})
}

// MarkAsRead marks messages from a user as read
// PUT /api/v1/chat/read/:senderID
func (h *ChatHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	senderID := c.Param("senderID")
	if senderID == "" {
		util.BadRequest(c, "sender_id is required")
		return
	}

	if err := h.chatService.MarkAsRead(userID.(string), senderID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Messages marked as read", nil)
}

// GetUnreadCount returns total unread message count
// GET /api/v1/chat/unread/count
func (h *ChatHandler) GetUnreadCount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	count, err := h.chatService.GetUnreadCount(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Unread count retrieved", gin.H{"count": count})
}

// GetUnreadCountBySenders returns unread count per sender (contact)
// GET /api/v1/chat/unread/by-senders
func (h *ChatHandler) GetUnreadCountBySenders(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	counts, err := h.chatService.GetUnreadCountBySenders(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Unread counts by sender retrieved", gin.H{"counts": counts})
}

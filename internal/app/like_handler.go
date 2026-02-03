package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	likeService service.LikeService
	jwtSecret   string
}

func NewLikeHandler(likeService service.LikeService, jwtSecret string) *LikeHandler {
	return &LikeHandler{
		likeService: likeService,
		jwtSecret:   jwtSecret,
	}
}

// LikePost handles liking a post
// POST /api/v1/posts/:id/like
func (h *LikeHandler) LikePost(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
		return
	}

	var req struct {
		Reaction string `json:"reaction,omitempty"` // like, love, haha, wow, sad, angry
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Reaction is optional, default to "like"
		req.Reaction = "like"
	}

	like, err := h.likeService.LikePost(userID.(string), postID, req.Reaction)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post liked successfully", gin.H{"like": like})
}

// UnlikePost handles unliking a post
// DELETE /api/v1/posts/:id/like
func (h *LikeHandler) UnlikePost(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
		return
	}

	err := h.likeService.UnlikePost(userID.(string), postID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post unliked successfully", nil)
}

// LikeComment handles liking a comment
// POST /api/v1/comments/:id/like
func (h *LikeHandler) LikeComment(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	commentID := c.Param("id")
	if commentID == "" {
		util.BadRequest(c, "Comment ID is required")
		return
	}

	var req struct {
		Reaction string `json:"reaction,omitempty"` // like, love, haha, wow, sad, angry
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Reaction is optional, default to "like"
		req.Reaction = "like"
	}

	like, err := h.likeService.LikeComment(userID.(string), commentID, req.Reaction)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment liked successfully", gin.H{"like": like})
}

// UnlikeComment handles unliking a comment
// DELETE /api/v1/comments/:id/like
func (h *LikeHandler) UnlikeComment(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	commentID := c.Param("id")
	if commentID == "" {
		util.BadRequest(c, "Comment ID is required")
		return
	}

	err := h.likeService.UnlikeComment(userID.(string), commentID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment unliked successfully", nil)
}

// GetLikes handles getting likes for a target
// GET /api/v1/likes?target_type=post&target_id=xxx&limit=20&offset=0
func (h *LikeHandler) GetLikes(c *gin.Context) {
	targetType := c.Query("target_type")
	targetID := c.Query("target_id")

	if targetType == "" || targetID == "" {
		util.BadRequest(c, "target_type and target_id are required")
		return
	}

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

	likes, total, err := h.likeService.GetLikesByTarget(targetType, targetID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Likes retrieved successfully", gin.H{
		"likes": likes,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}

// GetLikeCount handles getting like count for a target
// GET /api/v1/likes/count?target_type=post&target_id=xxx
func (h *LikeHandler) GetLikeCount(c *gin.Context) {
	targetType := c.Query("target_type")
	targetID := c.Query("target_id")

	if targetType == "" || targetID == "" {
		util.BadRequest(c, "target_type and target_id are required")
		return
	}

	count, err := h.likeService.GetLikeCount(targetType, targetID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Like count retrieved successfully", gin.H{"count": count})
}

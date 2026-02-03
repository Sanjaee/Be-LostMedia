package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	commentService service.CommentService
	jwtSecret      string
}

func NewCommentHandler(commentService service.CommentService, jwtSecret string) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		jwtSecret:      jwtSecret,
	}
}

// CreateComment handles comment creation
// POST /api/v1/comments
func (h *CommentHandler) CreateComment(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	comment, err := h.commentService.CreateComment(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Comment created successfully", gin.H{"comment": comment})
}

// GetComment handles getting a comment by ID
// GET /api/v1/comments/:id
func (h *CommentHandler) GetComment(c *gin.Context) {
	commentID := c.Param("id")
	if commentID == "" {
		util.BadRequest(c, "Comment ID is required")
		return
	}

	comment, err := h.commentService.GetCommentByID(commentID)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment retrieved successfully", gin.H{"comment": comment})
}

// GetCommentsByPost handles getting comments by post ID
// GET /api/v1/posts/:id/comments
func (h *CommentHandler) GetCommentsByPost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
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

	comments, total, err := h.commentService.GetCommentsByPostID(postID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comments retrieved successfully", gin.H{
		"comments": comments,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetReplies handles getting replies to a comment
// GET /api/v1/comments/:id/replies
func (h *CommentHandler) GetReplies(c *gin.Context) {
	commentID := c.Param("id")
	if commentID == "" {
		util.BadRequest(c, "Comment ID is required")
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

	replies, total, err := h.commentService.GetRepliesByCommentID(commentID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Replies retrieved successfully", gin.H{
		"replies": replies,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// UpdateComment handles comment update
// PUT /api/v1/comments/:id
func (h *CommentHandler) UpdateComment(c *gin.Context) {
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

	var req service.UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	comment, err := h.commentService.UpdateComment(userID.(string), commentID, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment updated successfully", gin.H{"comment": comment})
}

// DeleteComment handles comment deletion
// DELETE /api/v1/comments/:id
func (h *CommentHandler) DeleteComment(c *gin.Context) {
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

	err := h.commentService.DeleteComment(userID.(string), commentID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment deleted successfully", nil)
}

// GetCommentCount handles getting comment count for a post
// GET /api/v1/posts/:id/comments/count
func (h *CommentHandler) GetCommentCount(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
		return
	}

	count, err := h.commentService.GetCommentCount(postID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Comment count retrieved successfully", gin.H{"count": count})
}

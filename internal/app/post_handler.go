package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type PostHandler struct {
	postService service.PostService
	jwtSecret   string
}

func NewPostHandler(postService service.PostService, jwtSecret string) *PostHandler {
	return &PostHandler{
		postService: postService,
		jwtSecret:   jwtSecret,
	}
}

// CreatePost handles post creation
// POST /api/v1/posts
func (h *PostHandler) CreatePost(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	post, err := h.postService.CreatePost(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Post created successfully", gin.H{"post": post})
}

// GetPost handles getting a post by ID
// GET /api/v1/posts/:id
func (h *PostHandler) GetPost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
		return
	}

	// Get viewer ID (if authenticated)
	viewerID := ""
	if userID, exists := c.Get("userID"); exists {
		viewerID = userID.(string)
	}

	post, err := h.postService.GetPostByID(postID, viewerID)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post retrieved successfully", gin.H{"post": post})
}

// GetPostsByUserID handles getting posts by user ID
// GET /api/v1/posts/user/:userID
func (h *PostHandler) GetPostsByUserID(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		util.BadRequest(c, "User ID is required")
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

	// Get viewer ID (if authenticated)
	viewerID := ""
	if viewer, exists := c.Get("userID"); exists {
		viewerID = viewer.(string)
	}

	posts, err := h.postService.GetPostsByUserID(userID, viewerID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Posts retrieved successfully", gin.H{
		"posts":  posts,
		"limit":  limit,
		"offset": offset,
	})
}

// GetPostsByGroupID handles getting posts by group ID
// GET /api/v1/posts/group/:groupID
func (h *PostHandler) GetPostsByGroupID(c *gin.Context) {
	groupID := c.Param("groupID")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
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

	// Get viewer ID (if authenticated)
	viewerID := ""
	if viewer, exists := c.Get("userID"); exists {
		viewerID = viewer.(string)
	}

	posts, err := h.postService.GetPostsByGroupID(groupID, viewerID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Posts retrieved successfully", gin.H{
		"posts":  posts,
		"limit":  limit,
		"offset": offset,
	})
}

// GetFeed handles getting feed posts
// GET /api/v1/posts/feed
func (h *PostHandler) GetFeed(c *gin.Context) {
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

	posts, err := h.postService.GetFeed(userID.(string), limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Feed retrieved successfully", gin.H{
		"posts":  posts,
		"limit":  limit,
		"offset": offset,
	})
}

// UpdatePost handles post update
// PUT /api/v1/posts/:id
func (h *PostHandler) UpdatePost(c *gin.Context) {
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

	var req service.UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	post, err := h.postService.UpdatePost(userID.(string), postID, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post updated successfully", gin.H{"post": post})
}

// DeletePost handles post deletion
// DELETE /api/v1/posts/:id
func (h *PostHandler) DeletePost(c *gin.Context) {
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

	err := h.postService.DeletePost(userID.(string), postID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post deleted successfully", nil)
}

// CountPostsByUserID handles counting posts by user ID
// GET /api/v1/posts/user/:userID/count
func (h *PostHandler) CountPostsByUserID(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	count, err := h.postService.CountPostsByUserID(userID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post count retrieved successfully", gin.H{"count": count})
}

// CountPostsByGroupID handles counting posts by group ID
// GET /api/v1/posts/group/:groupID/count
func (h *PostHandler) CountPostsByGroupID(c *gin.Context) {
	groupID := c.Param("groupID")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	count, err := h.postService.CountPostsByGroupID(groupID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Post count retrieved successfully", gin.H{"count": count})
}

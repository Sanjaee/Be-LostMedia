package app

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"yourapp/internal/model"
	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type PostHandler struct {
	postService         service.PostService
	postViewService     service.PostViewService
	notificationService service.NotificationService
	cloudinaryClient    *util.CloudinaryClient
	wsHub               interface {
		BroadcastToUser(string, map[string]interface{})
	}
	jwtSecret string
}

func NewPostHandler(postService service.PostService, jwtSecret string) *PostHandler {
	return &PostHandler{
		postService: postService,
		jwtSecret:   jwtSecret,
	}
}

func NewPostHandlerWithCloudinary(
	postService service.PostService,
	postViewService service.PostViewService,
	notificationService service.NotificationService,
	cloudinaryClient *util.CloudinaryClient,
	wsHub interface {
		BroadcastToUser(string, map[string]interface{})
	},
	jwtSecret string,
) *PostHandler {
	return &PostHandler{
		postService:         postService,
		postViewService:     postViewService,
		notificationService: notificationService,
		cloudinaryClient:    cloudinaryClient,
		wsHub:               wsHub,
		jwtSecret:           jwtSecret,
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
	sortBy := c.DefaultQuery("sort", "newest") // newest or popular

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

	var posts []*model.Post
	if sortBy == "popular" {
		posts, err = h.postService.GetFeedByEngagement(userID.(string), limit, offset)
	} else {
		posts, err = h.postService.GetFeed(userID.(string), limit, offset)
	}
	
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Feed retrieved successfully", gin.H{
		"posts":  posts,
		"limit":  limit,
		"offset": offset,
		"sort":   sortBy,
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

// CreatePostWithImages handles post creation with image uploads (async)
// POST /api/v1/posts/upload
func (h *PostHandler) CreatePostWithImages(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		util.BadRequest(c, "Failed to parse form data")
		return
	}

	// Get content
	content := c.PostForm("content")
	var contentPtr *string
	if content != "" {
		contentPtr = &content
	}

	// Get group_id if provided
	groupID := c.PostForm("group_id")
	var groupIDPtr *string
	if groupID != "" {
		groupIDPtr = &groupID
	}

	// Get files
	form, err := c.MultipartForm()
	if err != nil {
		util.BadRequest(c, "Failed to parse multipart form")
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		files = form.File["files"]
	}

	// Validate: must have either content or images
	if (contentPtr == nil || *contentPtr == "") && len(files) == 0 {
		util.BadRequest(c, "Post must have either content or images")
		return
	}

	// Validate maximum 3 images
	if len(files) > 3 {
		util.BadRequest(c, "Maximum 3 images allowed")
		return
	}

	// Validate file sizes (max 3MB each)
	maxSize := int64(3 * 1024 * 1024) // 3MB
	for _, fileHeader := range files {
		if fileHeader.Size > maxSize {
			util.BadRequest(c, fmt.Sprintf("File %s exceeds 3MB limit", fileHeader.Filename))
			return
		}
	}

	// Create post immediately with empty image URLs (will be updated async)
	createReq := service.CreatePostRequest{
		Content:   contentPtr,
		ImageURLs: []string{}, // Empty initially, will be updated after processing
		GroupID:   groupIDPtr,
	}

	post, err := h.postService.CreatePost(userID.(string), createReq)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// If no images, return immediately
	if len(files) == 0 {
		util.SuccessResponse(c, http.StatusCreated, "Post created successfully", gin.H{"post": post})
		return
	}

	// Process images in background (async)
	go func() {
		var fileDataList []util.FileData

		// Read all files into memory
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Error opening file %s: %v", fileHeader.Filename, err)
				continue
			}

			fileData, err := util.ReadFileFromReader(file, fileHeader.Filename)
			file.Close()
			if err != nil {
				log.Printf("Error reading file %s: %v", fileHeader.Filename, err)
				continue
			}

			fileDataList = append(fileDataList, *fileData)
		}

		if len(fileDataList) == 0 {
			log.Printf("No valid files processed for post %s", post.ID)
			return
		}

		// Process and upload images
		imageURLs, err := h.cloudinaryClient.ProcessMultipleFiles(fileDataList)
		if err != nil {
			log.Printf("Error processing images for post %s: %v", post.ID, err)
			return
		}

		// Update post with image URLs
		updateReq := service.UpdatePostRequest{
			ImageURLs: imageURLs,
		}

		updatedPost, err := h.postService.UpdatePost(userID.(string), post.ID, updateReq)
		if err != nil {
			log.Printf("Error updating post %s with image URLs: %v", post.ID, err)
			return
		}

		log.Printf("Post %s processing completed with %d images", post.ID, len(imageURLs))

		// Send notification to user that upload is completed (saves to DB and sends via WebSocket)
		if h.notificationService != nil {
			if err := h.notificationService.SendPostUploadCompletedNotification(userID.(string), updatedPost.ID, len(imageURLs)); err != nil {
				log.Printf("Error sending post upload completed notification: %v", err)
			}
		}

		log.Printf("Post %s is ready with images", updatedPost.ID)
	}()

	// Send initial WebSocket notification that upload is pending
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(userID.(string), map[string]interface{}{
			"type":    "post_upload_pending",
			"post_id": post.ID,
			"message": "Post sedang diproses, gambar sedang diupload...",
			"status":  "pending",
			"data": map[string]interface{}{
				"post_id": post.ID,
			},
		})
	}

	// Return response immediately (ASYNC)
	util.SuccessResponse(c, http.StatusAccepted, "Post created and images are being processed", gin.H{
		"post":   post,
		"status": "processing",
	})
}

// TrackView handles tracking a post view
// POST /api/v1/posts/:id/view
func (h *PostHandler) TrackView(c *gin.Context) {
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

	err := h.postViewService.TrackView(userID.(string), postID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "View tracked successfully", nil)
}

// GetViewCount handles getting view count for a post
// GET /api/v1/posts/:id/views/count
func (h *PostHandler) GetViewCount(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		util.BadRequest(c, "Post ID is required")
		return
	}

	count, err := h.postViewService.GetViewCount(postID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "View count retrieved successfully", gin.H{"count": count})
}

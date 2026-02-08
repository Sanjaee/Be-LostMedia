package app

import (
	"fmt"
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	groupService service.GroupService
	cloudinary   *util.CloudinaryClient
	jwtSecret    string
}

func NewGroupHandler(groupService service.GroupService, cloudinary *util.CloudinaryClient, jwtSecret string) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
		cloudinary:   cloudinary,
		jwtSecret:    jwtSecret,
	}
}

// CreateGroup handles creating a new group
// POST /api/v1/groups
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "Group name is required")
		return
	}

	group, err := h.groupService.CreateGroup(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Group created successfully", gin.H{"group": group})
}

// CreateGroupWithCover handles creating a group with cover photo upload
// POST /api/v1/groups/upload
func (h *GroupHandler) CreateGroupWithCover(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		util.BadRequest(c, "Failed to parse form data")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		util.BadRequest(c, "Group name is required")
		return
	}

	description := c.PostForm("description")
	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	privacy := c.PostForm("privacy")
	if privacy == "" {
		privacy = "open"
	}

	membershipPolicy := c.PostForm("membership_policy")
	if membershipPolicy == "" {
		membershipPolicy = "anyone_can_join"
	}

	req := service.CreateGroupRequest{
		Name:             name,
		Description:      descPtr,
		Privacy:          privacy,
		MembershipPolicy: membershipPolicy,
	}

	// Handle cover photo upload
	file, err := c.FormFile("cover_photo")
	if err == nil && file != nil && h.cloudinary != nil {
		// Save file temporarily
		tmpPath := fmt.Sprintf("/tmp/group_cover_%s_%s", userID.(string), file.Filename)
		if err := c.SaveUploadedFile(file, tmpPath); err == nil {
			url, err := h.cloudinary.UploadImage(tmpPath)
			if err == nil {
				req.CoverPhoto = &url
			}
		}
	}

	group, err := h.groupService.CreateGroup(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Group created successfully", gin.H{"group": group})
}

// GetGroup handles getting a group by ID
// GET /api/v1/groups/:id
func (h *GroupHandler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	group, err := h.groupService.GetGroupByID(id)
	if err != nil {
		util.NotFound(c, "Group not found")
		return
	}

	// Check if current user is a member
	var isMember bool
	var memberRole string
	if userID, exists := c.Get("userID"); exists {
		member, err := h.groupService.GetMember(id, userID.(string))
		if err == nil && member != nil && member.Status == "active" {
			isMember = true
			memberRole = member.Role
		}
	}

	util.SuccessResponse(c, http.StatusOK, "Group retrieved successfully", gin.H{
		"group":       group,
		"is_member":   isMember,
		"member_role": memberRole,
	})
}

// GetGroupBySlug handles getting a group by slug
// GET /api/v1/groups/slug/:slug
func (h *GroupHandler) GetGroupBySlug(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		util.BadRequest(c, "Group slug is required")
		return
	}

	group, err := h.groupService.GetGroupBySlug(slug)
	if err != nil {
		util.NotFound(c, "Group not found")
		return
	}

	// Check if current user is a member
	var isMember bool
	var memberRole string
	if userID, exists := c.Get("userID"); exists {
		member, err := h.groupService.GetMember(group.ID, userID.(string))
		if err == nil && member != nil && member.Status == "active" {
			isMember = true
			memberRole = member.Role
		}
	}

	util.SuccessResponse(c, http.StatusOK, "Group retrieved successfully", gin.H{
		"group":       group,
		"is_member":   isMember,
		"member_role": memberRole,
	})
}

// UpdateGroup handles updating a group
// PUT /api/v1/groups/:id
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	var req service.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "Invalid request body")
		return
	}

	group, err := h.groupService.UpdateGroup(userID.(string), groupID, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Group updated successfully", gin.H{"group": group})
}

// DeleteGroup handles deleting a group
// DELETE /api/v1/groups/:id
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	if err := h.groupService.DeleteGroup(userID.(string), groupID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Group deleted successfully", nil)
}

// ListGroups handles listing all groups
// GET /api/v1/groups
func (h *GroupHandler) ListGroups(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	groups, total, err := h.groupService.ListGroups(limit, offset)
	if err != nil {
		util.InternalServerError(c, "Failed to list groups")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Groups listed successfully", gin.H{
		"groups": groups,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// SearchGroups handles searching groups
// GET /api/v1/groups/search?q=keyword
func (h *GroupHandler) SearchGroups(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		util.BadRequest(c, "Search query is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	groups, total, err := h.groupService.SearchGroups(keyword, limit, offset)
	if err != nil {
		util.InternalServerError(c, "Failed to search groups")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Groups found", gin.H{
		"groups": groups,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMyGroups handles getting groups the current user is a member of
// GET /api/v1/groups/my
func (h *GroupHandler) GetMyGroups(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	groups, total, err := h.groupService.GetUserGroups(userID.(string), limit, offset)
	if err != nil {
		util.InternalServerError(c, "Failed to get your groups")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Your groups", gin.H{
		"groups": groups,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// JoinGroup handles joining a group
// POST /api/v1/groups/:id/join
func (h *GroupHandler) JoinGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	member, err := h.groupService.JoinGroup(userID.(string), groupID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	message := "Joined group successfully"
	if member.Status == "pending" {
		message = "Join request sent. Waiting for admin approval"
	}

	util.SuccessResponse(c, http.StatusOK, message, gin.H{"member": member})
}

// LeaveGroup handles leaving a group
// POST /api/v1/groups/:id/leave
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	deleted, err := h.groupService.LeaveGroup(userID.(string), groupID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	payload := gin.H{}
	if deleted {
		payload["deleted"] = true
	}
	util.SuccessResponse(c, http.StatusOK, "Left group successfully", payload)
}

// GetMembers handles getting group members
// GET /api/v1/groups/:id/members
func (h *GroupHandler) GetMembers(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	members, total, err := h.groupService.GetMembers(groupID, limit, offset)
	if err != nil {
		util.InternalServerError(c, "Failed to get members")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Members listed", gin.H{
		"members": members,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// UpdateMemberRole handles updating a member's role
// PUT /api/v1/groups/:id/members/:userID/role
func (h *GroupHandler) UpdateMemberRole(c *gin.Context) {
	adminID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	targetUserID := c.Param("userID")

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "Role is required")
		return
	}

	if err := h.groupService.UpdateMemberRole(adminID.(string), groupID, targetUserID, req.Role); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Member role updated", nil)
}

// RemoveMember handles removing a member from a group
// DELETE /api/v1/groups/:id/members/:userID
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	adminID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	targetUserID := c.Param("userID")

	if err := h.groupService.RemoveMember(adminID.(string), groupID, targetUserID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Member removed", nil)
}

// UpdateGroupCover handles uploading/updating group cover photo
// PUT /api/v1/groups/:id/cover
func (h *GroupHandler) UpdateGroupCover(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		util.BadRequest(c, "Group ID is required")
		return
	}

	if h.cloudinary == nil {
		util.InternalServerError(c, "Image upload not configured")
		return
	}

	file, err := c.FormFile("cover_photo")
	if err != nil {
		util.BadRequest(c, "Cover photo file is required")
		return
	}

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		util.BadRequest(c, "File size exceeds 10MB limit")
		return
	}

	// Save temporarily
	tmpPath := fmt.Sprintf("/tmp/group_cover_%s_%s", groupID, file.Filename)
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		util.InternalServerError(c, "Failed to save uploaded file")
		return
	}

	// Upload to Cloudinary
	url, err := h.cloudinary.UploadImage(tmpPath)
	if err != nil {
		util.InternalServerError(c, "Failed to upload image")
		return
	}

	// Update group
	coverURL := url
	req := service.UpdateGroupRequest{
		CoverPhoto: &coverURL,
	}

	group, err := h.groupService.UpdateGroup(userID.(string), groupID, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Cover photo updated", gin.H{"group": group})
}

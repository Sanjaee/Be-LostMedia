package app

import (
	"net/http"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	profileService service.ProfileService
	jwtSecret      string
}

func NewProfileHandler(profileService service.ProfileService, jwtSecret string) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
		jwtSecret:      jwtSecret,
	}
}

// CreateProfile handles profile creation
// POST /api/v1/profiles
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	profile, err := h.profileService.CreateProfile(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Profile created successfully", gin.H{"profile": profile})
}

// GetProfile handles getting a profile by ID
// GET /api/v1/profiles/:id
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	profileID := c.Param("id")
	if profileID == "" {
		util.BadRequest(c, "Profile ID is required")
		return
	}

	profile, err := h.profileService.GetProfileByID(profileID)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{"profile": profile})
}

// GetProfileByUsername handles getting a profile by username (slug)
// GET /api/v1/profiles/username/:username
func (h *ProfileHandler) GetProfileByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		util.BadRequest(c, "Username is required")
		return
	}

	profile, err := h.profileService.GetProfileByUsername(username)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{"profile": profile})
}

// GetProfileByUserID handles getting a profile by user ID
// GET /api/v1/profiles/user/:userID
func (h *ProfileHandler) GetProfileByUserID(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		util.BadRequest(c, "User ID is required")
		return
	}

	profile, err := h.profileService.GetProfileByUserID(userID)
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{"profile": profile})
}

// GetMyProfile handles getting current user's profile
// GET /api/v1/profiles/me
func (h *ProfileHandler) GetMyProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	profile, err := h.profileService.GetMyProfile(userID.(string))
	if err != nil {
		util.NotFound(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{"profile": profile})
}

// UpdateProfile handles profile update
// PUT /api/v1/profiles/:id
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	profileID := c.Param("id")
	if profileID == "" {
		util.BadRequest(c, "Profile ID is required")
		return
	}

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	profile, err := h.profileService.UpdateProfile(userID.(string), profileID, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile updated successfully", gin.H{"profile": profile})
}

// DeleteProfile handles profile deletion
// DELETE /api/v1/profiles/:id
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	profileID := c.Param("id")
	if profileID == "" {
		util.BadRequest(c, "Profile ID is required")
		return
	}

	err := h.profileService.DeleteProfile(userID.(string), profileID)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Profile deleted successfully", nil)
}

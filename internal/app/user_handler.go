package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/repository"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo  repository.UserRepository
	jwtSecret string
}

func NewUserHandler(userRepo repository.UserRepository, jwtSecret string) *UserHandler {
	return &UserHandler{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
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

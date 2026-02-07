package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	authService service.AuthService
	jwtSecret   string
}

func NewAuthHandler(authService service.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtSecret:   jwtSecret,
	}
}

// Register handles user registration
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.Register(req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, resp.Message, resp)
}

// Login handles user login
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.Login(req)
	if err != nil {
		if strings.Contains(err.Error(), "not verified") {
			// Return special response for unverified email with email in data
			util.ErrorResponse(c, http.StatusUnauthorized, err.Error(), gin.H{
				"email":                 req.Email,
				"requires_verification": true,
				"message":               "OTP telah dikirim ke email Anda. Silakan verifikasi email untuk melanjutkan.",
			})
			return
		}
		util.Unauthorized(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Login successful", resp)
}

// VerifyOTP handles OTP verification
// POST /api/v1/auth/verify-otp
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req struct {
		Email   string `json:"email" binding:"required,email"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.VerifyOTP(req.Email, req.OTPCode)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "OTP verified successfully", resp)
}

// ResendOTP handles OTP resend
// POST /api/v1/auth/resend-otp
func (h *AuthHandler) ResendOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	if err := h.authService.ResendOTP(req.Email); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "OTP sent successfully", nil)
}

// GoogleOAuth handles Google OAuth login
// POST /api/v1/auth/google-oauth
func (h *AuthHandler) GoogleOAuth(c *gin.Context) {
	var req service.GoogleOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.GoogleOAuth(req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Google OAuth successful", resp)
}

// RefreshToken handles token refresh
// POST /api/v1/auth/refresh-token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		util.Unauthorized(c, err.Error())
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Token refreshed successfully", resp)
}

// RequestResetPassword handles password reset request
// POST /api/v1/auth/forgot-password
func (h *AuthHandler) RequestResetPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	if err := h.authService.RequestResetPassword(req.Email); err != nil {
		// Return error if email doesn't exist or other error occurs
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Kode OTP telah dikirim ke email Anda", nil)
}

// VerifyResetPassword handles password reset with OTP verification
// POST /api/v1/auth/verify-reset-password
func (h *AuthHandler) VerifyResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		OTPCode     string `json:"otp_code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Parse validation errors and provide user-friendly messages
		var validationErr validator.ValidationErrors
		if errors.As(err, &validationErr) {
			for _, fieldErr := range validationErr {
				switch fieldErr.Field() {
				case "NewPassword":
					if fieldErr.Tag() == "min" {
						util.BadRequest(c, "Password minimal 8 karakter")
						return
					} else if fieldErr.Tag() == "max" {
						util.BadRequest(c, "Password maksimal 128 karakter")
						return
					}
				case "Email":
					util.BadRequest(c, "Format email tidak valid")
					return
				case "OTPCode":
					util.BadRequest(c, "Kode OTP wajib diisi")
					return
				}
			}
		}
		// Check for specific error messages
		errStr := err.Error()
		if strings.Contains(errStr, "min") && strings.Contains(errStr, "NewPassword") {
			util.BadRequest(c, "Password minimal 8 karakter")
			return
		}
		if strings.Contains(errStr, "max") && strings.Contains(errStr, "NewPassword") {
			util.BadRequest(c, "Password maksimal 128 karakter")
			return
		}
		util.BadRequest(c, errStr)
		return
	}

	if err := h.authService.VerifyResetPassword(req.Email, req.OTPCode, req.NewPassword); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Password reset successfully. Please login with your new password.", nil)
}

// ResetPassword handles password reset with token
// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.ResetPassword(req.Token, req.NewPassword)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Password reset successfully", resp)
}

// VerifyEmail handles email verification
// POST /api/v1/auth/verify-email
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	resp, err := h.authService.VerifyEmail(req.Token)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Email verified successfully", resp)
}

// GetMe handles getting current user info
// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.authService.GetMe(userID.(string))
	if err != nil {
		util.NotFound(c, "User not found")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "User retrieved successfully", gin.H{"user": user})
}

// DeleteAccount handles account deletion
// DELETE /api/v1/auth/account
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req struct {
		Password string `json:"password"` // Required for credential login
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.authService.DeleteAccount(userID.(string), req.Password); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Account deleted successfully", nil)
}

// SearchUsers handles searching users by keyword
// GET /api/v1/users/search?q=keyword&limit=20&offset=0
func (h *AuthHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		util.BadRequest(c, "Search keyword is required")
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

	users, err := h.authService.SearchUsers(keyword, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Users retrieved successfully", gin.H{
		"users":  users,
		"limit":  limit,
		"offset": offset,
		"total":  len(users),
	})
}

// AuthMiddleware validates JWT token
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			util.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			util.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := util.ValidateToken(token, h.jwtSecret)
		if err != nil {
			util.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("userType", claims.UserType)
		c.Next()
	}
}

// AdminMiddleware validates that the user is an owner
func (h *AuthHandler) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First check authentication
		if _, exists := c.Get("userID"); !exists {
			util.Unauthorized(c, "User not authenticated")
			c.Abort()
			return
		}

		// Get user type
		userType, exists := c.Get("userType")
		if !exists {
			util.Unauthorized(c, "User type not found")
			c.Abort()
			return
		}

		// Check if user is owner
		if userType != "owner" {
			util.ErrorResponse(c, http.StatusForbidden, "Access denied: Owner role required", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

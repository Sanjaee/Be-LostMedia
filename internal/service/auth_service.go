package service

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"yourapp/internal/config"
	"yourapp/internal/model"
	"yourapp/internal/repository"
	"yourapp/internal/util"
)

type AuthService interface {
	Register(req RegisterRequest) (*RegisterResponse, error)
	Login(req LoginRequest) (*AuthResponse, error)
	VerifyOTP(email, otpCode string) (*AuthResponse, error)
	ResendOTP(email string) (*ResendOTPResult, error)
	GetOTPResendStatus(email string) (*ResendOTPResult, error)
	GoogleOAuth(req GoogleOAuthRequest) (*AuthResponse, error)
	RefreshToken(refreshToken string) (*AuthResponse, error)
	RequestResetPassword(email string, isResend bool) (*ResendOTPResult, error)
	VerifyResetPassword(email, otpCode, newPassword string) error
	ResetPassword(token, newPassword string) (*AuthResponse, error)
	VerifyEmail(token string) (*AuthResponse, error)
	GetMe(userID string) (*model.User, error)
	SearchUsers(keyword string, limit, offset int) ([]model.User, error)
	DeleteAccount(userID string, password string) error
	UnbanUser(userID string) error
}

type authService struct {
	userRepo  repository.UserRepository
	jwtSecret string
	rabbitMQ  *util.RabbitMQClient
	config    *config.Config
}

type RegisterRequest struct {
	FullName    string  `json:"full_name" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	Username    *string `json:"username,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	Password    string  `json:"password" binding:"required,min=8"`
	UserType    string  `json:"user_type"`
	Gender      *string `json:"gender,omitempty"`
	DateOfBirth *string `json:"date_of_birth,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type GoogleOAuthRequest struct {
	Email        string `json:"email" binding:"required,email"`
	FullName     string `json:"full_name" binding:"required"`
	ProfilePhoto string `json:"profile_photo"`
	GoogleID     string `json:"google_id" binding:"required"`
}

type RegisterResponse struct {
	Message              string      `json:"message"`
	User                 *model.User `json:"user"`
	RequiresVerification bool        `json:"requires_verification"`
	VerificationToken    *string     `json:"verification_token,omitempty"`
}

type AuthResponse struct {
	User         *model.User `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int         `json:"expires_in"`
}

// ResendOTPResult holds next_resend_at (Unix) for FE countdown
type ResendOTPResult struct {
	NextResendAt int64 `json:"next_resend_at"` // Unix timestamp when user can resend again
	CanResend    bool  `json:"can_resend"`
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, rabbitMQ *util.RabbitMQClient) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		rabbitMQ:  rabbitMQ,
		config:    nil, // Will be set if needed
	}
}

// NewAuthServiceWithConfig creates auth service with config for RabbitMQ reconnection
func NewAuthServiceWithConfig(userRepo repository.UserRepository, jwtSecret string, rabbitMQ *util.RabbitMQClient, cfg *config.Config) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		rabbitMQ:  rabbitMQ,
		config:    cfg,
	}
}

// ensureRabbitMQ ensures RabbitMQ connection is available, reconnects if needed
func (s *authService) ensureRabbitMQ() {
	if s.rabbitMQ != nil {
		// Check if connection is still valid
		if s.rabbitMQ.GetChannel() != nil && !s.rabbitMQ.GetChannel().IsClosed() {
			return
		}
	}

	// Try to reconnect if config is available
	if s.config != nil {
		log.Println("Attempting to reconnect RabbitMQ...")
		newRabbitMQ, err := util.NewRabbitMQClient(s.config)
		if err == nil {
			s.rabbitMQ = newRabbitMQ
			log.Println("RabbitMQ reconnected successfully")
		} else {
			log.Printf("Failed to reconnect RabbitMQ: %v", err)
		}
	}
}

func (s *authService) Register(req RegisterRequest) (*RegisterResponse, error) {
	// Check if email already exists
	existingUser, _ := s.userRepo.FindByEmail(req.Email)
	if existingUser != nil {
		if existingUser.LoginType == "google" {
			return nil, errors.New("email already registered with Google. Please use Google Sign In")
		}
		return nil, errors.New("email already registered with password. Please login with email and password")
	}

	// Check username if provided
	if req.Username != nil && *req.Username != "" {
		existingUsername, _ := s.userRepo.FindByUsername(*req.Username)
		if existingUsername != nil {
			return nil, errors.New("username already taken")
		}
	}

	// Hash password
	passwordHash, err := util.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate OTP
	otpCode := generateOTP()
	otpExpiresAt := time.Now().Add(10 * time.Minute)

	// Parse date of birth if provided
	var dob *time.Time
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err == nil {
			dob = &parsed
		}
	}

	userType := req.UserType
	if userType == "" {
		userType = "member"
	}

	// Create user
	user := &model.User{
		Email:        req.Email,
		Username:     req.Username,
		Phone:        req.Phone,
		FullName:     req.FullName,
		PasswordHash: passwordHash,
		UserType:     userType,
		Gender:       req.Gender,
		DateOfBirth:  dob,
		IsActive:     true,
		IsVerified:   false,
		LoginType:    "credential",
		OTPCode:      &otpCode,
		OTPExpiresAt: &otpExpiresAt,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send OTP email via RabbitMQ asynchronously (non-blocking)
	// Same logic as reset password - send to queue immediately without waiting
	go func() {
		s.ensureRabbitMQ() // Try to reconnect if needed
		if s.rabbitMQ != nil {
			emailMsg := util.EmailMessage{
				To:      req.Email,
				Subject: "Kode Verifikasi Email",
				Body:    otpCode,
				Type:    "otp",
			}
			if err := s.rabbitMQ.PublishEmail(emailMsg); err != nil {
				// Log error but don't fail registration
				log.Printf("Failed to publish OTP email: %v\n", err)
			} else {
				log.Printf("OTP email queued successfully for %s", req.Email)
			}
		} else {
			log.Printf("Warning: RabbitMQ not available, OTP email not sent for %s", req.Email)
		}
	}()

	// Return immediately without waiting for email to be sent
	return &RegisterResponse{
		Message:              "Registration successful. Please verify your email with OTP.",
		User:                 user,
		RequiresVerification: true,
	}, nil
}

func (s *authService) Login(req LoginRequest) (*AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	// Check if user registered with Google instead of credential
	if user.LoginType == "google" {
		return nil, errors.New("email sudah terdaftar dengan Google. Silakan login dengan Google")
	}

	// Check if user registered with credential
	if user.LoginType != "credential" {
		return nil, errors.New("invalid email or password")
	}

	// Check password
	if !util.CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("account is deactivated")
	}

	// Check if email is verified
	if !user.IsVerified {
		// Generate new OTP
		otpCode := generateOTP()
		otpExpiresAt := time.Now().Add(10 * time.Minute)
		s.userRepo.UpdateOTP(req.Email, otpCode, otpExpiresAt)

		// Send OTP email via RabbitMQ asynchronously (non-blocking)
		go func() {
			s.ensureRabbitMQ() // Try to reconnect if needed
			if s.rabbitMQ != nil {
				emailMsg := util.EmailMessage{
					To:      req.Email,
					Subject: "Kode Verifikasi Email",
					Body:    otpCode,
					Type:    "otp",
				}
				if err := s.rabbitMQ.PublishEmail(emailMsg); err != nil {
					log.Printf("Failed to publish OTP email: %v\n", err)
				} else {
					log.Printf("OTP email queued successfully for %s", req.Email)
				}
			} else {
				log.Printf("Warning: RabbitMQ not available, OTP email not sent for %s", req.Email)
			}
		}()

		return nil, errors.New("email not verified. Please verify your email first")
	}

	// Update last login
	s.userRepo.UpdateLastLogin(user.ID)

	// Generate tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
	}, nil
}

func (s *authService) VerifyOTP(email, otpCode string) (*AuthResponse, error) {
	user, err := s.userRepo.VerifyOTP(email, otpCode)
	if err != nil {
		return nil, err
	}

	// Update last login
	s.userRepo.UpdateLastLogin(user.ID)

	// Generate tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}, nil
}

// getOTPResendCooldown returns duration after n-th resend: 1st=30s, 2nd=5m, 3rd=3h, 4th+=3d
func getOTPResendCooldown(nthResend int) time.Duration {
	switch nthResend {
	case 1:
		return 30 * time.Second
	case 2:
		return 5 * time.Minute
	case 3:
		return 3 * time.Hour
	default:
		return 3 * 24 * time.Hour // 3 days (4th and beyond)
	}
}

func (s *authService) GetOTPResendStatus(email string) (*ResendOTPResult, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	return s.computeResendStatus(user.OTPResendCount, user.OTPLastResendAt)
}

func (s *authService) computeResendStatus(resendCount int, lastResendAt *time.Time) (*ResendOTPResult, error) {
	if lastResendAt == nil {
		return &ResendOTPResult{CanResend: true, NextResendAt: 0}, nil
	}
	cooldown := getOTPResendCooldown(resendCount) // resendCount = nth resend done
	nextResendAt := lastResendAt.Add(cooldown)
	now := time.Now()
	if now.After(nextResendAt) || now.Equal(nextResendAt) {
		return &ResendOTPResult{CanResend: true, NextResendAt: 0}, nil
	}
	return &ResendOTPResult{
		CanResend:    false,
		NextResendAt: nextResendAt.Unix(),
	}, nil
}

func (s *authService) ResendOTP(email string) (*ResendOTPResult, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	// Check cooldown
	resendCount := user.OTPResendCount
	lastResendAt := user.OTPLastResendAt
	if lastResendAt != nil {
		cooldown := getOTPResendCooldown(resendCount) // cooldown after last resend
		nextResendAt := lastResendAt.Add(cooldown)
		if time.Now().Before(nextResendAt) {
			return &ResendOTPResult{
				CanResend:    false,
				NextResendAt: nextResendAt.Unix(),
			}, errors.New("silakan tunggu sebelum mengirim ulang")
		}
	}

	// Generate new OTP
	otpCode := generateOTP()
	otpExpiresAt := time.Now().Add(10 * time.Minute)
	newResendCount := resendCount + 1

	if err := s.userRepo.UpdateOTPWithResend(email, otpCode, otpExpiresAt, newResendCount); err != nil {
		return nil, fmt.Errorf("failed to update OTP: %w", err)
	}

	// Send OTP email via RabbitMQ asynchronously
	go func() {
		s.ensureRabbitMQ()
		if s.rabbitMQ != nil {
			emailMsg := util.EmailMessage{
				To:      email,
				Subject: "Kode Verifikasi Email",
				Body:    otpCode,
				Type:    "otp",
			}
			if err := s.rabbitMQ.PublishEmail(emailMsg); err != nil {
				log.Printf("Failed to publish OTP email: %v\n", err)
			} else {
				log.Printf("OTP email queued successfully for %s", email)
			}
		} else {
			log.Printf("Warning: RabbitMQ not available, OTP email not sent for %s", email)
		}
	}()

	nextCooldown := getOTPResendCooldown(newResendCount) // cooldown after this resend
	nextResendAt := time.Now().Add(nextCooldown)
	return &ResendOTPResult{
		CanResend:    false,
		NextResendAt: nextResendAt.Unix(),
	}, nil
}

func (s *authService) GoogleOAuth(req GoogleOAuthRequest) (*AuthResponse, error) {
	// Check if user exists by Google ID
	user, err := s.userRepo.FindByGoogleID(req.GoogleID)
	if err == nil {
		// User exists, update and return tokens
		user.LastLogin = &[]time.Time{time.Now()}[0]
		s.userRepo.UpdateLastLogin(user.ID)

		accessToken, _ := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
		refreshToken, _ := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)

		return &AuthResponse{
			User:         user,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    900,
		}, nil
	}

	// Check if email already exists
	existingUser, _ := s.userRepo.FindByEmail(req.Email)
	if existingUser != nil {
		// Check if user registered with credential instead of Google
		if existingUser.LoginType == "credential" {
			return nil, errors.New("email sudah terdaftar dengan email dan password. Silakan login dengan email dan password")
		}
		if existingUser.GoogleID != nil && *existingUser.GoogleID != req.GoogleID {
			return nil, errors.New("email already registered with different Google account")
		}
	}

	// Create new user
	user = &model.User{
		Email:        req.Email,
		FullName:     req.FullName,
		ProfilePhoto: &req.ProfilePhoto,
		UserType:     "member",
		IsActive:     true,
		IsVerified:   true, // Google users are auto-verified
		LoginType:    "google",
		GoogleID:     &req.GoogleID,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}, nil
}

func (s *authService) RefreshToken(refreshToken string) (*AuthResponse, error) {
	claims, err := util.ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    900,
	}, nil
}

func (s *authService) RequestResetPassword(email string, isResend bool) (*ResendOTPResult, error) {
	// Check if email exists in database first - must exist before sending email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil || user == nil {
		// Email doesn't exist in database - return error
		return nil, errors.New("email tidak terdaftar di sistem")
	}
	if user.LoginType != "credential" {
		return nil, errors.New("reset password hanya tersedia untuk akun yang terdaftar dengan email dan password")
	}

	otpCode := generateOTP()
	otpExpiresAt := time.Now().Add(10 * time.Minute)

	if isResend {
		if user.OTPLastResendAt != nil {
			cooldown := getOTPResendCooldown(user.OTPResendCount)
			nextResendAt := user.OTPLastResendAt.Add(cooldown)
			if time.Now().Before(nextResendAt) {
				return &ResendOTPResult{
					CanResend:    false,
					NextResendAt: nextResendAt.Unix(),
				}, errors.New("silakan tunggu sebelum mengirim ulang")
			}
		}
		newResendCount := user.OTPResendCount + 1
		if err := s.userRepo.UpdateOTPWithResend(email, otpCode, otpExpiresAt, newResendCount); err != nil {
			return nil, fmt.Errorf("failed to update OTP: %w", err)
		}
		s.sendResetPasswordOTP(email, otpCode)
		nextCooldown := getOTPResendCooldown(newResendCount)
		nextResendAt := time.Now().Add(nextCooldown)
		return &ResendOTPResult{
			CanResend:    false,
			NextResendAt: nextResendAt.Unix(),
		}, nil
	}

	if err := s.userRepo.UpdateOTP(email, otpCode, otpExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to update OTP: %w", err)
	}
	s.sendResetPasswordOTP(email, otpCode)
	return &ResendOTPResult{CanResend: true, NextResendAt: 0}, nil
}

func (s *authService) sendResetPasswordOTP(email, otpCode string) {
	go func() {
		s.ensureRabbitMQ()
		if s.rabbitMQ != nil {
			emailMsg := util.EmailMessage{
				To:      email,
				Subject: "Kode Reset Password",
				Body:    otpCode,
				Type:    "reset_password",
			}
			if err := s.rabbitMQ.PublishEmail(emailMsg); err != nil {
				log.Printf("Failed to publish reset password OTP email: %v\n", err)
			} else {
				log.Printf("Reset password OTP email queued successfully for %s", email)
			}
		} else {
			log.Printf("Warning: RabbitMQ not available, reset password OTP email not sent for %s", email)
		}
	}()
}

func (s *authService) VerifyResetPassword(email, otpCode, newPassword string) error {
	// First, verify that email exists in database and check login type before OTP verification
	existingUser, err := s.userRepo.FindByEmail(email)
	if err != nil || existingUser == nil {
		// Email doesn't exist in database - return error
		return errors.New("invalid or expired OTP")
	}

	// Check if user registered with credential (not Google) - must be credential to reset password
	if existingUser.LoginType != "credential" {
		return errors.New("reset password hanya tersedia untuk akun yang terdaftar dengan email dan password")
	}

	// Verify OTP code - this will also validate email, OTP, and expiry
	user, err := s.userRepo.VerifyOTP(email, otpCode)
	if err != nil {
		return errors.New("invalid or expired OTP")
	}

	// Double check login type after OTP verification (should be same, but extra security)
	if user.LoginType != "credential" {
		return errors.New("reset password hanya tersedia untuk akun yang terdaftar dengan email dan password")
	}

	// Hash new password
	passwordHash, err := util.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password and clear OTP (OTP already cleared by VerifyOTP)
	if err := s.userRepo.UpdatePassword(user.ID, passwordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (s *authService) ResetPassword(token, newPassword string) (*AuthResponse, error) {
	// Validate JWT token first
	claims, err := util.ValidateToken(token, s.jwtSecret)
	if err != nil {
		return nil, errors.New("invalid or expired reset token")
	}

	// Verify token is for reset password (userType should be "reset")
	if claims.UserType != "reset" {
		return nil, errors.New("invalid reset token")
	}

	// Find user by ID from token
	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Verify reset token matches database
	if user.ResetToken == nil || *user.ResetToken != token {
		return nil, errors.New("invalid reset token")
	}

	// Check if token is expired
	if user.ResetExpiresAt == nil || user.ResetExpiresAt.Before(time.Now()) {
		return nil, errors.New("reset token has expired")
	}

	// Hash new password
	passwordHash, err := util.HashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password and clear reset token
	if err := s.userRepo.UpdatePassword(user.ID, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Generate tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}, nil
}

func (s *authService) VerifyEmail(token string) (*AuthResponse, error) {
	// For now, treat token as OTP code
	// In production, you might want to use JWT token
	claims, err := util.ValidateToken(token, s.jwtSecret)
	if err != nil {
		// If token validation fails, try as OTP
		// This is a simplified approach - in production, use proper email verification tokens
		return nil, errors.New("invalid verification token")
	}

	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	user.IsVerified = true
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to verify user: %w", err)
	}

	// Generate tokens
	accessToken, err := util.GenerateAccessToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := util.GenerateRefreshToken(user.ID, user.Email, user.UserType, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}, nil
}

func (s *authService) GetMe(userID string) (*model.User, error) {
	return s.userRepo.FindByID(userID)
}

// SearchUsers searches users by keyword
func (s *authService) SearchUsers(keyword string, limit, offset int) ([]model.User, error) {
	if keyword == "" {
		return []model.User{}, nil
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.userRepo.SearchUsers(keyword, limit, offset)
}

// DeleteAccount soft-deletes the user account
func (s *authService) DeleteAccount(userID string, password string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	// For credential login, verify password
	if user.LoginType == "credential" && user.PasswordHash != "" {
		if password == "" {
			return errors.New("password required to delete account")
		}
		if !util.CheckPasswordHash(password, user.PasswordHash) {
			return errors.New("invalid password")
		}
	}

	return s.userRepo.Delete(userID)
}

func (s *authService) UnbanUser(userID string) error {
	return s.userRepo.UnbanUser(userID)
}

// generateOTP generates a 6-digit OTP
func generateOTP() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

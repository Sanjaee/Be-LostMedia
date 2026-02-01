package util

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	UserType string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken generates a JWT token
func GenerateToken(userID, email, userType, secret string, expiresIn time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Email:    email,
		UserType: userType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "yourapp",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateAccessToken generates an access token (15 minutes)
func GenerateAccessToken(userID, email, userType, secret string) (string, error) {
	return GenerateToken(userID, email, userType, secret, 15*time.Minute)
}

// GenerateRefreshToken generates a refresh token (7 days)
func GenerateRefreshToken(userID, email, userType, secret string) (string, error) {
	return GenerateToken(userID, email, userType, secret, 7*24*time.Hour)
}

// GenerateResetPasswordToken generates a reset password token (1 hour)
func GenerateResetPasswordToken(userID, email, secret string) (string, error) {
	return GenerateToken(userID, email, "reset", secret, 1*time.Hour)
}

// ValidateToken validates a JWT token
func ValidateToken(tokenString, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

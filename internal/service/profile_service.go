package service

import (
	"errors"
	"fmt"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type ProfileService interface {
	CreateProfile(userID string, req CreateProfileRequest) (*model.Profile, error)
	GetProfileByID(profileID string) (*model.Profile, error)
	GetProfileByUserID(userID string) (*model.Profile, error)
	UpdateProfile(userID string, profileID string, req UpdateProfileRequest) (*model.Profile, error)
	DeleteProfile(userID string, profileID string) error
	GetMyProfile(userID string) (*model.Profile, error)
}

type profileService struct {
	profileRepo repository.ProfileRepository
	userRepo    repository.UserRepository
}

type CreateProfileRequest struct {
	Bio                *string `json:"bio,omitempty"`
	CoverPhoto         *string `json:"cover_photo,omitempty"`
	Website            *string `json:"website,omitempty"`
	Location           *string `json:"location,omitempty"`
	City               *string `json:"city,omitempty"`
	Country            *string `json:"country,omitempty"`
	Hometown           *string `json:"hometown,omitempty"`
	Education          *string `json:"education,omitempty"`
	Work               *string `json:"work,omitempty"`
	RelationshipStatus *string `json:"relationship_status,omitempty"`
	Intro              *string `json:"intro,omitempty"`
	IsProfilePublic    *bool   `json:"is_profile_public,omitempty"`
}

type UpdateProfileRequest struct {
	Bio                *string `json:"bio,omitempty"`
	CoverPhoto         *string `json:"cover_photo,omitempty"`
	Website            *string `json:"website,omitempty"`
	Location           *string `json:"location,omitempty"`
	City               *string `json:"city,omitempty"`
	Country            *string `json:"country,omitempty"`
	Hometown           *string `json:"hometown,omitempty"`
	Education          *string `json:"education,omitempty"`
	Work               *string `json:"work,omitempty"`
	RelationshipStatus *string `json:"relationship_status,omitempty"`
	Intro              *string `json:"intro,omitempty"`
	IsProfilePublic    *bool   `json:"is_profile_public,omitempty"`
}

func NewProfileService(profileRepo repository.ProfileRepository, userRepo repository.UserRepository) ProfileService {
	return &profileService{
		profileRepo: profileRepo,
		userRepo:    userRepo,
	}
}

// CreateProfile creates a new profile for a user
func (s *profileService) CreateProfile(userID string, req CreateProfileRequest) (*model.Profile, error) {
	// Check if user exists
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Check if profile already exists
	existingProfile, err := s.profileRepo.FindByUserID(userID)
	if err == nil && existingProfile != nil {
		return nil, errors.New("profile already exists for this user")
	}

	// Create new profile
	profile := &model.Profile{
		UserID:             userID,
		Bio:                req.Bio,
		CoverPhoto:         req.CoverPhoto,
		Website:            req.Website,
		Location:           req.Location,
		City:               req.City,
		Country:            req.Country,
		Hometown:           req.Hometown,
		Education:          req.Education,
		Work:               req.Work,
		RelationshipStatus: req.RelationshipStatus,
		Intro:              req.Intro,
		IsProfilePublic:    true, // default
	}

	if req.IsProfilePublic != nil {
		profile.IsProfilePublic = *req.IsProfilePublic
	}

	if err := s.profileRepo.Create(profile); err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Reload with user relationship
	return s.profileRepo.FindByID(profile.ID)
}

// GetProfileByID retrieves a profile by ID
func (s *profileService) GetProfileByID(profileID string) (*model.Profile, error) {
	profile, err := s.profileRepo.FindByID(profileID)
	if err != nil {
		return nil, errors.New("profile not found")
	}

	// Check if profile is public or user is viewing their own profile
	if !profile.IsProfilePublic {
		return nil, errors.New("profile is private")
	}

	return profile, nil
}

// GetProfileByUserID retrieves a profile by user ID
func (s *profileService) GetProfileByUserID(userID string) (*model.Profile, error) {
	profile, err := s.profileRepo.FindByUserID(userID)
	if err != nil {
		return nil, errors.New("profile not found")
	}

	return profile, nil
}

// GetMyProfile retrieves the current user's profile
func (s *profileService) GetMyProfile(userID string) (*model.Profile, error) {
	profile, err := s.profileRepo.FindByUserID(userID)
	if err != nil {
		return nil, errors.New("profile not found")
	}

	return profile, nil
}

// UpdateProfile updates a profile
func (s *profileService) UpdateProfile(userID string, profileID string, req UpdateProfileRequest) (*model.Profile, error) {
	// Get existing profile
	profile, err := s.profileRepo.FindByID(profileID)
	if err != nil {
		return nil, errors.New("profile not found")
	}

	// Check if user owns this profile
	if profile.UserID != userID {
		return nil, errors.New("unauthorized: you can only update your own profile")
	}

	// Update fields
	if req.Bio != nil {
		profile.Bio = req.Bio
	}
	if req.CoverPhoto != nil {
		profile.CoverPhoto = req.CoverPhoto
	}
	if req.Website != nil {
		profile.Website = req.Website
	}
	if req.Location != nil {
		profile.Location = req.Location
	}
	if req.City != nil {
		profile.City = req.City
	}
	if req.Country != nil {
		profile.Country = req.Country
	}
	if req.Hometown != nil {
		profile.Hometown = req.Hometown
	}
	if req.Education != nil {
		profile.Education = req.Education
	}
	if req.Work != nil {
		profile.Work = req.Work
	}
	if req.RelationshipStatus != nil {
		profile.RelationshipStatus = req.RelationshipStatus
	}
	if req.Intro != nil {
		profile.Intro = req.Intro
	}
	if req.IsProfilePublic != nil {
		profile.IsProfilePublic = *req.IsProfilePublic
	}

	if err := s.profileRepo.Update(profile); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Reload with user relationship
	return s.profileRepo.FindByID(profile.ID)
}

// DeleteProfile deletes a profile
func (s *profileService) DeleteProfile(userID string, profileID string) error {
	// Get existing profile
	profile, err := s.profileRepo.FindByID(profileID)
	if err != nil {
		return errors.New("profile not found")
	}

	// Check if user owns this profile
	if profile.UserID != userID {
		return errors.New("unauthorized: you can only delete your own profile")
	}

	if err := s.profileRepo.Delete(profileID); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	return nil
}

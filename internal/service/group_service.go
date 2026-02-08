package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type GroupService interface {
	CreateGroup(userID string, req CreateGroupRequest) (*model.Group, error)
	GetGroupByID(id string) (*model.Group, error)
	GetGroupBySlug(slug string) (*model.Group, error)
	UpdateGroup(userID, groupID string, req UpdateGroupRequest) (*model.Group, error)
	DeleteGroup(userID, groupID string) error
	ListGroups(limit, offset int) ([]model.Group, int64, error)
	SearchGroups(keyword string, limit, offset int) ([]model.Group, int64, error)
	GetUserGroups(userID string, limit, offset int) ([]model.Group, int64, error)

	// Membership
	JoinGroup(userID, groupID string) (*model.GroupMember, error)
	LeaveGroup(userID, groupID string) (deleted bool, err error)
	GetMembers(groupID string, limit, offset int) ([]model.GroupMember, int64, error)
	GetMember(groupID, userID string) (*model.GroupMember, error)
	UpdateMemberRole(adminID, groupID, targetUserID, role string) error
	RemoveMember(adminID, groupID, targetUserID string) error
	IsMember(groupID, userID string) (bool, error)
}

type groupService struct {
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
}

func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository) GroupService {
	return &groupService{
		groupRepo: groupRepo,
		userRepo:  userRepo,
	}
}

type CreateGroupRequest struct {
	Name             string  `json:"name" binding:"required"`
	Description      *string `json:"description"`
	CoverPhoto       *string `json:"cover_photo"`
	Icon             *string `json:"icon"`
	Privacy          string  `json:"privacy"`           // open, closed, secret
	MembershipPolicy string  `json:"membership_policy"` // anyone_can_join, approval_required
}

type UpdateGroupRequest struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	CoverPhoto       *string `json:"cover_photo"`
	Icon             *string `json:"icon"`
	Privacy          *string `json:"privacy"`
	MembershipPolicy *string `json:"membership_policy"`
}

// generateSlug generates a URL-friendly slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "group"
	}
	return slug
}

// CreateGroup creates a new group and adds the creator as admin
func (s *groupService) CreateGroup(userID string, req CreateGroupRequest) (*model.Group, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("group name is required")
	}

	// Validate privacy
	privacy := req.Privacy
	if privacy == "" {
		privacy = "open"
	}
	if privacy != "open" && privacy != "closed" && privacy != "secret" {
		return nil, errors.New("invalid privacy setting. Must be open, closed, or secret")
	}

	// Validate membership policy
	policy := req.MembershipPolicy
	if policy == "" {
		policy = "anyone_can_join"
	}
	if policy != "anyone_can_join" && policy != "approval_required" {
		return nil, errors.New("invalid membership policy")
	}

	// Generate unique slug
	baseSlug := generateSlug(req.Name)
	slug := baseSlug
	for i := 1; ; i++ {
		taken, err := s.groupRepo.IsSlugTaken(slug)
		if err != nil {
			return nil, fmt.Errorf("failed to check slug: %w", err)
		}
		if !taken {
			break
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}

	group := &model.Group{
		CreatedBy:        userID,
		Name:             strings.TrimSpace(req.Name),
		Slug:             slug,
		Description:      req.Description,
		CoverPhoto:       req.CoverPhoto,
		Icon:             req.Icon,
		Privacy:          privacy,
		MembershipPolicy: policy,
		IsActive:         true,
	}

	if err := s.groupRepo.CreateGroup(group); err != nil {
		// Return user-friendly message for duplicate slug (name already in use)
		if strings.Contains(err.Error(), "groups_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") ||
			strings.Contains(err.Error(), "23505") {
			return nil, errors.New("nama grup sudah digunakan")
		}
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	// Add creator as admin member
	member := &model.GroupMember{
		GroupID: group.ID,
		UserID:  userID,
		Role:    "admin",
		Status:  "active",
	}
	if err := s.groupRepo.AddMember(member); err != nil {
		return nil, fmt.Errorf("failed to add creator as admin: %w", err)
	}

	// Re-fetch group with preloaded data
	return s.groupRepo.FindGroupByID(group.ID)
}

func (s *groupService) GetGroupByID(id string) (*model.Group, error) {
	return s.groupRepo.FindGroupByID(id)
}

func (s *groupService) GetGroupBySlug(slug string) (*model.Group, error) {
	return s.groupRepo.FindGroupBySlug(slug)
}

func (s *groupService) UpdateGroup(userID, groupID string, req UpdateGroupRequest) (*model.Group, error) {
	group, err := s.groupRepo.FindGroupByID(groupID)
	if err != nil {
		return nil, errors.New("group not found")
	}

	// Check if user is admin of the group
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil || member.Role != "admin" {
		return nil, errors.New("only group admins can update the group")
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		group.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		group.Description = req.Description
	}
	if req.CoverPhoto != nil {
		group.CoverPhoto = req.CoverPhoto
	}
	if req.Icon != nil {
		group.Icon = req.Icon
	}
	if req.Privacy != nil {
		p := *req.Privacy
		if p != "open" && p != "closed" && p != "secret" {
			return nil, errors.New("invalid privacy setting")
		}
		group.Privacy = p
	}
	if req.MembershipPolicy != nil {
		mp := *req.MembershipPolicy
		if mp != "anyone_can_join" && mp != "approval_required" {
			return nil, errors.New("invalid membership policy")
		}
		group.MembershipPolicy = mp
	}

	if err := s.groupRepo.UpdateGroup(group); err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	return s.groupRepo.FindGroupByID(groupID)
}

func (s *groupService) DeleteGroup(userID, groupID string) error {
	// Check if user is admin of the group
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil || member.Role != "admin" {
		return errors.New("only group admins can delete the group")
	}

	return s.groupRepo.DeleteGroup(groupID)
}

func (s *groupService) ListGroups(limit, offset int) ([]model.Group, int64, error) {
	return s.groupRepo.ListGroups(limit, offset)
}

func (s *groupService) SearchGroups(keyword string, limit, offset int) ([]model.Group, int64, error) {
	return s.groupRepo.SearchGroups(keyword, limit, offset)
}

func (s *groupService) GetUserGroups(userID string, limit, offset int) ([]model.Group, int64, error) {
	return s.groupRepo.GetUserGroups(userID, limit, offset)
}

// JoinGroup lets a user join a group
func (s *groupService) JoinGroup(userID, groupID string) (*model.GroupMember, error) {
	// Check if group exists
	group, err := s.groupRepo.FindGroupByID(groupID)
	if err != nil {
		return nil, errors.New("group not found")
	}

	if !group.IsActive {
		return nil, errors.New("group is not active")
	}

	// Check if already a member
	existing, _ := s.groupRepo.GetMember(groupID, userID)
	if existing != nil {
		if existing.Status == "active" {
			return nil, errors.New("already a member of this group")
		}
		if existing.Status == "banned" {
			return nil, errors.New("you have been banned from this group")
		}
		// If pending, just return
		if existing.Status == "pending" {
			return existing, nil
		}
	}

	status := "active"
	if group.MembershipPolicy == "approval_required" {
		status = "pending"
	}

	member := &model.GroupMember{
		GroupID: groupID,
		UserID:  userID,
		Role:    "member",
		Status:  status,
	}

	if err := s.groupRepo.AddMember(member); err != nil {
		return nil, fmt.Errorf("failed to join group: %w", err)
	}

	return member, nil
}

func (s *groupService) LeaveGroup(userID, groupID string) (bool, error) {
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return false, errors.New("you are not a member of this group")
	}

	// If admin is the only admin, delete the group when they leave
	if member.Role == "admin" {
		members, _, _ := s.groupRepo.GetMembers(groupID, 1000, 0)
		adminCount := 0
		for _, m := range members {
			if m.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			err := s.groupRepo.DeleteGroup(groupID)
			return err == nil, err
		}
	}

	return false, s.groupRepo.RemoveMember(groupID, userID)
}

func (s *groupService) GetMembers(groupID string, limit, offset int) ([]model.GroupMember, int64, error) {
	return s.groupRepo.GetMembers(groupID, limit, offset)
}

func (s *groupService) GetMember(groupID, userID string) (*model.GroupMember, error) {
	return s.groupRepo.GetMember(groupID, userID)
}

func (s *groupService) UpdateMemberRole(adminID, groupID, targetUserID, role string) error {
	if role != "admin" && role != "moderator" && role != "member" {
		return errors.New("invalid role. Must be admin, moderator, or member")
	}

	// Check if requester is admin
	adminMember, err := s.groupRepo.GetMember(groupID, adminID)
	if err != nil || adminMember.Role != "admin" {
		return errors.New("only group admins can change member roles")
	}

	// Cannot change own role
	if adminID == targetUserID {
		return errors.New("cannot change your own role")
	}

	return s.groupRepo.UpdateMemberRole(groupID, targetUserID, role)
}

func (s *groupService) RemoveMember(adminID, groupID, targetUserID string) error {
	// Check if requester is admin or moderator
	adminMember, err := s.groupRepo.GetMember(groupID, adminID)
	if err != nil || (adminMember.Role != "admin" && adminMember.Role != "moderator") {
		return errors.New("only group admins or moderators can remove members")
	}

	// Cannot remove yourself
	if adminID == targetUserID {
		return errors.New("cannot remove yourself. Use leave group instead")
	}

	// Moderator cannot remove admin
	targetMember, err := s.groupRepo.GetMember(groupID, targetUserID)
	if err != nil {
		return errors.New("target user is not a member")
	}
	if adminMember.Role == "moderator" && targetMember.Role == "admin" {
		return errors.New("moderators cannot remove admins")
	}

	return s.groupRepo.RemoveMember(groupID, targetUserID)
}

func (s *groupService) IsMember(groupID, userID string) (bool, error) {
	return s.groupRepo.IsMember(groupID, userID)
}

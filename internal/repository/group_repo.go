package repository

import (
	"yourapp/internal/model"
	"yourapp/internal/util"

	"gorm.io/gorm"
)

type GroupRepository interface {
	// Group CRUD
	CreateGroup(group *model.Group) error
	FindGroupByID(id string) (*model.Group, error)
	FindGroupBySlug(slug string) (*model.Group, error)
	UpdateGroup(group *model.Group) error
	DeleteGroup(id string) error
	ListGroups(limit, offset int) ([]model.Group, int64, error)
	SearchGroups(keyword string, limit, offset int) ([]model.Group, int64, error)
	IsSlugTaken(slug string) (bool, error)

	// Group Members
	AddMember(member *model.GroupMember) error
	RemoveMember(groupID, userID string) error
	GetMember(groupID, userID string) (*model.GroupMember, error)
	GetMembers(groupID string, limit, offset int) ([]model.GroupMember, int64, error)
	UpdateMemberRole(groupID, userID, role string) error
	UpdateMemberStatus(groupID, userID, status string) error
	CountMembers(groupID string) (int64, error)
	GetUserGroups(userID string, limit, offset int) ([]model.Group, int64, error)
	IsMember(groupID, userID string) (bool, error)
}

type groupRepository struct {
	db    *gorm.DB
	redis *util.RedisClient
}

func NewGroupRepository(db *gorm.DB, redis *util.RedisClient) GroupRepository {
	return &groupRepository{db: db, redis: redis}
}

// CreateGroup creates a new group
func (r *groupRepository) CreateGroup(group *model.Group) error {
	return r.db.Create(group).Error
}

// FindGroupByID finds a group by its ID
func (r *groupRepository) FindGroupByID(id string) (*model.Group, error) {
	var group model.Group
	err := r.db.Preload("Creator").Where("id = ?", id).First(&group).Error
	if err != nil {
		return nil, err
	}

	// Count members
	var count int64
	r.db.Model(&model.GroupMember{}).Where("group_id = ? AND status = ?", id, "active").Count(&count)
	group.MembersCount = count

	return &group, nil
}

// FindGroupBySlug finds a group by its slug
func (r *groupRepository) FindGroupBySlug(slug string) (*model.Group, error) {
	var group model.Group
	err := r.db.Preload("Creator").Where("slug = ?", slug).First(&group).Error
	if err != nil {
		return nil, err
	}

	// Count members
	var count int64
	r.db.Model(&model.GroupMember{}).Where("group_id = ? AND status = ?", group.ID, "active").Count(&count)
	group.MembersCount = count

	return &group, nil
}

// UpdateGroup updates a group
func (r *groupRepository) UpdateGroup(group *model.Group) error {
	return r.db.Save(group).Error
}

// DeleteGroup soft-deletes a group
func (r *groupRepository) DeleteGroup(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.Group{}).Error
}

// ListGroups lists all active groups with pagination
func (r *groupRepository) ListGroups(limit, offset int) ([]model.Group, int64, error) {
	var groups []model.Group
	var total int64

	if err := r.db.Model(&model.Group{}).Where("is_active = ?", true).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Preload("Creator").
		Where("is_active = ?", true).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	// Count members for each group
	for i := range groups {
		var count int64
		r.db.Model(&model.GroupMember{}).Where("group_id = ? AND status = ?", groups[i].ID, "active").Count(&count)
		groups[i].MembersCount = count
	}

	return groups, total, nil
}

// SearchGroups searches groups by keyword
func (r *groupRepository) SearchGroups(keyword string, limit, offset int) ([]model.Group, int64, error) {
	var groups []model.Group
	var total int64
	pattern := "%" + keyword + "%"

	query := r.db.Model(&model.Group{}).
		Where("is_active = ?", true).
		Where("name ILIKE ? OR description ILIKE ?", pattern, pattern)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Preload("Creator").
		Where("is_active = ?", true).
		Where("name ILIKE ? OR description ILIKE ?", pattern, pattern).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	for i := range groups {
		var count int64
		r.db.Model(&model.GroupMember{}).Where("group_id = ? AND status = ?", groups[i].ID, "active").Count(&count)
		groups[i].MembersCount = count
	}

	return groups, total, nil
}

// IsSlugTaken checks if a slug is already used
func (r *groupRepository) IsSlugTaken(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Group{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}

// AddMember adds a member to a group
func (r *groupRepository) AddMember(member *model.GroupMember) error {
	return r.db.Create(member).Error
}

// RemoveMember removes a member from a group
func (r *groupRepository) RemoveMember(groupID, userID string) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&model.GroupMember{}).Error
}

// GetMember gets a specific member of a group
func (r *groupRepository) GetMember(groupID, userID string) (*model.GroupMember, error) {
	var member model.GroupMember
	err := r.db.Preload("User").
		Where("group_id = ? AND user_id = ?", groupID, userID).
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetMembers gets all members of a group with pagination
func (r *groupRepository) GetMembers(groupID string, limit, offset int) ([]model.GroupMember, int64, error) {
	var members []model.GroupMember
	var total int64

	if err := r.db.Model(&model.GroupMember{}).
		Where("group_id = ? AND status = ?", groupID, "active").
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Preload("User").
		Where("group_id = ? AND status = ?", groupID, "active").
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&members).Error
	if err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

// UpdateMemberRole updates a member's role
func (r *groupRepository) UpdateMemberRole(groupID, userID, role string) error {
	return r.db.Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("role", role).Error
}

// UpdateMemberStatus updates a member's status
func (r *groupRepository) UpdateMemberStatus(groupID, userID, status string) error {
	return r.db.Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Update("status", status).Error
}

// CountMembers counts active members of a group
func (r *groupRepository) CountMembers(groupID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.GroupMember{}).
		Where("group_id = ? AND status = ?", groupID, "active").
		Count(&count).Error
	return count, err
}

// GetUserGroups gets all groups a user is a member of
func (r *groupRepository) GetUserGroups(userID string, limit, offset int) ([]model.Group, int64, error) {
	var groups []model.Group
	var total int64

	subQuery := r.db.Model(&model.GroupMember{}).
		Select("group_id").
		Where("user_id = ? AND status = ?", userID, "active")

	if err := r.db.Model(&model.Group{}).
		Where("id IN (?) AND is_active = ?", subQuery, true).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Preload("Creator").
		Where("id IN (?) AND is_active = ?", subQuery, true).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	for i := range groups {
		var count int64
		r.db.Model(&model.GroupMember{}).Where("group_id = ? AND status = ?", groups[i].ID, "active").Count(&count)
		groups[i].MembersCount = count
	}

	return groups, total, nil
}

// IsMember checks if a user is an active member of a group
func (r *groupRepository) IsMember(groupID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ? AND status = ?", groupID, userID, "active").
		Count(&count).Error
	return count > 0, err
}

package repository

import (
	"strings"

	"yourapp/internal/model"

	"gorm.io/gorm"
)

type RolePriceRepository interface {
	Create(rolePrice *model.RolePrice) error
	FindByID(id string) (*model.RolePrice, error)
	FindByRole(role string) (*model.RolePrice, error)
	FindAll(includeInactive bool) ([]model.RolePrice, error)
	Update(rolePrice *model.RolePrice) error
	Delete(id string) error
	ExistsByRole(role string, excludeID string) (bool, error)
}

type rolePriceRepository struct {
	db *gorm.DB
}

func NewRolePriceRepository(db *gorm.DB) RolePriceRepository {
	return &rolePriceRepository{db: db}
}

func (r *rolePriceRepository) Create(rolePrice *model.RolePrice) error {
	return r.db.Create(rolePrice).Error
}

func (r *rolePriceRepository) FindByID(id string) (*model.RolePrice, error) {
	var rolePrice model.RolePrice
	err := r.db.Where("id = ?", id).First(&rolePrice).Error
	if err != nil {
		return nil, err
	}
	return &rolePrice, nil
}

func (r *rolePriceRepository) FindByRole(role string) (*model.RolePrice, error) {
	var rolePrice model.RolePrice
	err := r.db.Where("LOWER(role) = ?", strings.ToLower(role)).First(&rolePrice).Error
	if err != nil {
		return nil, err
	}
	return &rolePrice, nil
}

func (r *rolePriceRepository) FindAll(includeInactive bool) ([]model.RolePrice, error) {
	var list []model.RolePrice
	query := r.db.Order("sort_order ASC, role ASC")
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.Find(&list).Error
	return list, err
}

func (r *rolePriceRepository) Update(rolePrice *model.RolePrice) error {
	return r.db.Save(rolePrice).Error
}

func (r *rolePriceRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.RolePrice{}).Error
}

func (r *rolePriceRepository) ExistsByRole(role string, excludeID string) (bool, error) {
	var count int64
	query := r.db.Model(&model.RolePrice{}).Where("LOWER(role) = ?", strings.ToLower(role))
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

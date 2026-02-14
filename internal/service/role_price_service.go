package service

import (
	"fmt"
	"strings"

	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type RolePriceService interface {
	Create(req CreateRolePriceRequest) (*model.RolePrice, error)
	GetByID(id string) (*model.RolePrice, error)
	GetByRole(role string) (*model.RolePrice, error)
	List(includeInactive bool) ([]model.RolePrice, error)
	Update(id string, req UpdateRolePriceRequest) (*model.RolePrice, error)
	Delete(id string) error
}

type rolePriceService struct {
	rolePriceRepo repository.RolePriceRepository
}

type CreateRolePriceRequest struct {
	Role        string `json:"role" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Price       int    `json:"price" binding:"required,min=0"`
	IsActive    bool   `json:"is_active"`
	SortOrder   int    `json:"sort_order"`
}

type UpdateRolePriceRequest struct {
	Role        *string `json:"role,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Price       *int    `json:"price,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	SortOrder   *int    `json:"sort_order,omitempty"`
}

func NewRolePriceService(rolePriceRepo repository.RolePriceRepository) RolePriceService {
	return &rolePriceService{rolePriceRepo: rolePriceRepo}
}

func (s *rolePriceService) Create(req CreateRolePriceRequest) (*model.RolePrice, error) {
	role := strings.TrimSpace(strings.ToLower(req.Role))
	if role == "" {
		return nil, fmt.Errorf("role is required")
	}
	if role == "owner" {
		return nil, fmt.Errorf("cannot set price for owner role")
	}

	exists, err := s.rolePriceRepo.ExistsByRole(role, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("role price for role '%s' already exists", role)
	}

	rp := &model.RolePrice{
		Role:        role,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		IsActive:    req.IsActive,
		SortOrder:   req.SortOrder,
	}

	if err := s.rolePriceRepo.Create(rp); err != nil {
		return nil, err
	}

	return s.rolePriceRepo.FindByID(rp.ID)
}

func (s *rolePriceService) GetByID(id string) (*model.RolePrice, error) {
	return s.rolePriceRepo.FindByID(id)
}

func (s *rolePriceService) GetByRole(role string) (*model.RolePrice, error) {
	return s.rolePriceRepo.FindByRole(role)
}

func (s *rolePriceService) List(includeInactive bool) ([]model.RolePrice, error) {
	return s.rolePriceRepo.FindAll(includeInactive)
}

func (s *rolePriceService) Update(id string, req UpdateRolePriceRequest) (*model.RolePrice, error) {
	rp, err := s.rolePriceRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.Role != nil {
		role := strings.TrimSpace(strings.ToLower(*req.Role))
		if role == "owner" {
			return nil, fmt.Errorf("cannot set owner role price")
		}
		exists, err := s.rolePriceRepo.ExistsByRole(role, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("role price for role '%s' already exists", role)
		}
		rp.Role = role
	}
	if req.Name != nil {
		rp.Name = *req.Name
	}
	if req.Description != nil {
		rp.Description = *req.Description
	}
	if req.Price != nil {
		if *req.Price < 0 {
			return nil, fmt.Errorf("price cannot be negative")
		}
		rp.Price = *req.Price
	}
	if req.IsActive != nil {
		rp.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		rp.SortOrder = *req.SortOrder
	}

	if err := s.rolePriceRepo.Update(rp); err != nil {
		return nil, err
	}

	return s.rolePriceRepo.FindByID(id)
}

func (s *rolePriceService) Delete(id string) error {
	_, err := s.rolePriceRepo.FindByID(id)
	if err != nil {
		return err
	}
	return s.rolePriceRepo.Delete(id)
}

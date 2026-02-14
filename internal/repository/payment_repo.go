package repository

import (
	"yourapp/internal/model"

	"gorm.io/gorm"
)

type PaymentRepository interface {
	Create(payment *model.Payment) error
	FindByID(id string) (*model.Payment, error)
	FindByOrderID(orderID string) (*model.Payment, error)
	FindByUserID(userID string, limit, offset int) ([]model.Payment, int64, error)
	Update(payment *model.Payment) error
	Updates(orderID string, updates map[string]interface{}) error
}

type paymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) Create(payment *model.Payment) error {
	return r.db.Create(payment).Error
}

func (r *paymentRepository) FindByID(id string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.Where("id = ?", id).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindByOrderID(orderID string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.Where("order_id = ?", orderID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *paymentRepository) FindByUserID(userID string, limit, offset int) ([]model.Payment, int64, error) {
	var payments []model.Payment
	var total int64

	query := r.db.Model(&model.Payment{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&payments).Error
	if err != nil {
		return nil, 0, err
	}

	return payments, total, nil
}

func (r *paymentRepository) Update(payment *model.Payment) error {
	return r.db.Save(payment).Error
}

func (r *paymentRepository) Updates(orderID string, updates map[string]interface{}) error {
	return r.db.Model(&model.Payment{}).Where("order_id = ?", orderID).Updates(updates).Error
}

package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"yourapp/internal/config"
	"yourapp/internal/model"
	"yourapp/internal/repository"
	"yourapp/internal/websocket"

	"github.com/google/uuid"
)

const (
	midtransBaseURLSandbox = "https://api.sandbox.midtrans.com/v2"
	midtransBaseURLProd    = "https://api.midtrans.com/v2"
)

type PaymentService interface {
	CreatePayment(userID string, req CreatePaymentRequest) (*model.Payment, error)
	CreatePaymentForRole(userID string, req CreatePaymentForRoleRequest) (*model.Payment, error)
	GetPaymentByID(id string) (*model.Payment, error)
	GetPaymentByOrderID(orderID string) (*model.Payment, error)
	GetPaymentsByUserID(userID string, limit, offset int) ([]model.Payment, int64, error)
	CheckPaymentStatusFromMidtrans(orderID string) error
	HandleWebhook(payload []byte) error
}

type paymentService struct {
	paymentRepo   repository.PaymentRepository
	rolePriceRepo repository.RolePriceRepository
	userRepo      repository.UserRepository
	cfg           *config.Config
	wsHub         *websocket.Hub
}

// CreatePaymentForRoleRequest untuk upgrade role - pakai harga dari role_prices
type CreatePaymentForRoleRequest struct {
	TargetRole    string `json:"target_role" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required,oneof=bank_transfer gopay credit_card qris"`
	Bank          string `json:"bank,omitempty"`
	CardTokenID   string `json:"card_token_id,omitempty"`
	SaveCard      bool   `json:"save_card,omitempty"`
}

type CreatePaymentRequest struct {
	Amount        int    `json:"amount" binding:"required,min=1000"`
	ItemName      string `json:"item_name" binding:"required"`
	ItemCategory  string `json:"item_category"`
	Description   string `json:"description"`
	PaymentMethod string `json:"payment_method" binding:"required,oneof=bank_transfer gopay credit_card qris"`
	Bank          string `json:"bank,omitempty"`
	CardTokenID   string `json:"card_token_id,omitempty"`
	SaveCard      bool   `json:"save_card,omitempty"`
	Metadata      string `json:"metadata,omitempty"`
	TargetRole    string `json:"target_role,omitempty"` // Jika diisi, pakai harga dari role_prices
}

// Midtrans API structs
type midtransChargeRequest struct {
	PaymentType        string                     `json:"payment_type"`
	TransactionDetails midtransTransactionDetails `json:"transaction_details"`
	CustomerDetails    midtransCustomerDetails    `json:"customer_details"`
	ItemDetails        []midtransItemDetail       `json:"item_details"`
	BankTransfer       *midtransBankTransfer      `json:"bank_transfer,omitempty"`
	Gopay              *midtransGopay             `json:"gopay,omitempty"`
	CreditCard         *midtransCreditCard        `json:"credit_card,omitempty"`
}

type midtransTransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int    `json:"gross_amount"`
}

type midtransCustomerDetails struct {
	FirstName string `json:"first_name"`
	Email     string `json:"email"`
}

type midtransItemDetail struct {
	ID       string `json:"id"`
	Price    int    `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type midtransBankTransfer struct {
	Bank string `json:"bank"`
}

type midtransGopay struct {
	EnableCallback bool   `json:"enable_callback"`
	CallbackURL    string `json:"callback_url"`
}

type midtransCreditCard struct {
	TokenID        string `json:"token_id"`
	Authentication bool   `json:"authentication"`
	Bank           string `json:"bank,omitempty"`
	SaveTokenID    bool   `json:"save_token_id,omitempty"`
}

type midtransChargeResponse struct {
	TransactionID     string             `json:"transaction_id"`
	OrderID           string             `json:"order_id"`
	GrossAmount       string             `json:"gross_amount"`
	PaymentType       string             `json:"payment_type"`
	TransactionTime   string             `json:"transaction_time"`
	TransactionStatus string             `json:"transaction_status"`
	FraudStatus       string             `json:"fraud_status"`
	StatusMessage     string             `json:"status_message"`
	VANumbers         []midtransVANumber `json:"va_numbers,omitempty"`
	Actions           []midtransAction   `json:"actions,omitempty"`
	ExpiryTime        string             `json:"expiry_time,omitempty"`
	RedirectURL       string             `json:"redirect_url,omitempty"`
	MaskedCard        string             `json:"masked_card,omitempty"`
	Bank              string             `json:"bank,omitempty"`
	CardType          string             `json:"card_type,omitempty"`
	SavedTokenID      string             `json:"saved_token_id,omitempty"`
}

type midtransVANumber struct {
	Bank     string `json:"bank"`
	VANumber string `json:"va_number"`
}

type midtransAction struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

func NewPaymentService(paymentRepo repository.PaymentRepository, rolePriceRepo repository.RolePriceRepository, userRepo repository.UserRepository, cfg *config.Config, wsHub *websocket.Hub) PaymentService {
	return &paymentService{
		paymentRepo:   paymentRepo,
		rolePriceRepo: rolePriceRepo,
		userRepo:      userRepo,
		cfg:           cfg,
		wsHub:         wsHub,
	}
}

func mapMidtransStatus(status string) model.PaymentStatus {
	switch status {
	case "pending":
		return model.PaymentStatusPending
	case "settlement", "capture":
		return model.PaymentStatusSuccess
	case "deny":
		return model.PaymentStatusFailed
	case "cancel":
		return model.PaymentStatusCancelled
	case "expire":
		return model.PaymentStatusExpired
	default:
		return model.PaymentStatusPending
	}
}

func (s *paymentService) getBaseURL() string {
	if s.cfg.MidtransIsProd {
		return midtransBaseURLProd
	}
	return midtransBaseURLSandbox
}

func (s *paymentService) CreatePaymentForRole(userID string, req CreatePaymentForRoleRequest) (*model.Payment, error) {
	rolePrice, err := s.rolePriceRepo.FindByRole(req.TargetRole)
	if err != nil {
		return nil, fmt.Errorf("role price for '%s' not found", req.TargetRole)
	}
	if !rolePrice.IsActive {
		return nil, fmt.Errorf("role '%s' is not available for purchase", req.TargetRole)
	}
	if rolePrice.Price <= 0 {
		return nil, fmt.Errorf("invalid price for role '%s'", req.TargetRole)
	}

	createReq := CreatePaymentRequest{
		Amount:        rolePrice.Price,
		ItemName:      fmt.Sprintf("Upgrade ke %s", rolePrice.Name),
		ItemCategory:  "role_upgrade",
		Description:   rolePrice.Description,
		PaymentMethod: req.PaymentMethod,
		Bank:          req.Bank,
		CardTokenID:   req.CardTokenID,
		SaveCard:      req.SaveCard,
		TargetRole:    strings.ToLower(req.TargetRole),
	}
	return s.CreatePayment(userID, createReq)
}

func (s *paymentService) CreatePayment(userID string, req CreatePaymentRequest) (*model.Payment, error) {
	if s.cfg.MidtransServerKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY is not configured")
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	orderID := fmt.Sprintf("PAY_%s", uuid.New().String())
	itemCategory := req.ItemCategory
	if itemCategory == "" {
		itemCategory = "digital"
	}

	chargeData := midtransChargeRequest{
		PaymentType: req.PaymentMethod,
		TransactionDetails: midtransTransactionDetails{
			OrderID:     orderID,
			GrossAmount: req.Amount,
		},
		CustomerDetails: midtransCustomerDetails{
			FirstName: user.FullName,
			Email:     user.Email,
		},
		ItemDetails: []midtransItemDetail{
			{
				ID:       "item-1",
				Price:    req.Amount,
				Quantity: 1,
				Name:     req.ItemName,
				Category: itemCategory,
			},
		},
	}

	callbackURL := fmt.Sprintf("%s/payment/callback", s.cfg.FrontendURL)
	if callbackURL == "/payment/callback" {
		callbackURL = s.cfg.ClientURL + "/payment/callback"
	}

	switch req.PaymentMethod {
	case "bank_transfer":
		bank := req.Bank
		if bank == "" {
			bank = "bca"
		}
		chargeData.BankTransfer = &midtransBankTransfer{Bank: bank}
	case "gopay":
		chargeData.Gopay = &midtransGopay{
			EnableCallback: true,
			CallbackURL:    callbackURL,
		}
	case "qris":
		chargeData.PaymentType = "qris"
		chargeData.Gopay = &midtransGopay{
			EnableCallback: true,
			CallbackURL:    callbackURL,
		}
	case "credit_card":
		if req.CardTokenID == "" {
			return nil, fmt.Errorf("card_token_id is required for credit card payment")
		}
		chargeData.CreditCard = &midtransCreditCard{
			TokenID:        req.CardTokenID,
			Authentication: true,
			SaveTokenID:    req.SaveCard,
		}
		if req.Bank != "" {
			chargeData.CreditCard.Bank = req.Bank
		}
	}

	payment := &model.Payment{
		ID:            uuid.New().String(),
		UserID:        userID,
		OrderID:       orderID,
		Amount:        req.Amount,
		TotalAmount:   req.Amount,
		Status:        model.PaymentStatusPending,
		PaymentMethod: req.PaymentMethod,
		PaymentType:   "midtrans",
		ItemName:      req.ItemName,
		ItemCategory:  itemCategory,
		Description:   req.Description,
		CustomerName:  user.FullName,
		CustomerEmail: user.Email,
		Metadata:      req.Metadata,
		TargetRole:    req.TargetRole,
	}

	if err := s.paymentRepo.Create(payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %v", err)
	}

	chargeJSON, err := json.Marshal(chargeData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal charge data: %v", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(s.cfg.MidtransServerKey + ":"))

	httpReq, err := http.NewRequest("POST", s.getBaseURL()+"/charge", bytes.NewBuffer(chargeJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to charge Midtrans: %v", err)
		return payment, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read Midtrans response: %v", err)
		return payment, nil
	}

	var midtransResp midtransChargeResponse
	if err := json.Unmarshal(body, &midtransResp); err != nil {
		log.Printf("Failed to parse Midtrans response: %v", err)
		return payment, nil
	}

	var vaNumber, bankType, qrCodeURL string
	if len(midtransResp.VANumbers) > 0 {
		vaNumber = midtransResp.VANumbers[0].VANumber
		bankType = midtransResp.VANumbers[0].Bank
	}

	for _, action := range midtransResp.Actions {
		if (action.Name == "generate-qr-code" || action.Name == "generate-qr-code-v2") && action.URL != "" {
			qrCodeURL = action.URL
			break
		}
	}
	if qrCodeURL == "" {
		for _, action := range midtransResp.Actions {
			if action.Method == "GET" && action.URL != "" {
				qrCodeURL = action.URL
				break
			}
		}
	}

	var expiryTime *time.Time
	if midtransResp.ExpiryTime != "" {
		formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
		for _, f := range formats {
			if exp, err := time.Parse(f, midtransResp.ExpiryTime); err == nil {
				expiryTime = &exp
				break
			}
		}
	}

	updates := map[string]interface{}{
		"midtrans_transaction_id": midtransResp.TransactionID,
		"status":                  mapMidtransStatus(midtransResp.TransactionStatus),
		"fraud_status":            midtransResp.FraudStatus,
		"midtrans_response":       string(body),
		"va_number":               vaNumber,
		"bank_type":               bankType,
		"qr_code_url":             qrCodeURL,
		"expiry_time":             expiryTime,
		"updated_at":              time.Now(),
	}

	if midtransResp.RedirectURL != "" {
		updates["redirect_url"] = midtransResp.RedirectURL
	}
	if midtransResp.MaskedCard != "" {
		updates["masked_card"] = midtransResp.MaskedCard
	}
	if midtransResp.CardType != "" {
		updates["card_type"] = midtransResp.CardType
	}
	if midtransResp.SavedTokenID != "" {
		updates["saved_token_id"] = midtransResp.SavedTokenID
	}
	if midtransResp.Bank != "" {
		updates["bank_type"] = midtransResp.Bank
	}

	if err := s.paymentRepo.Updates(orderID, updates); err != nil {
		log.Printf("Failed to update payment: %v", err)
	}

	payment, _ = s.paymentRepo.FindByOrderID(orderID)
	return payment, nil
}

func (s *paymentService) GetPaymentByID(id string) (*model.Payment, error) {
	return s.paymentRepo.FindByID(id)
}

func (s *paymentService) GetPaymentByOrderID(orderID string) (*model.Payment, error) {
	return s.paymentRepo.FindByOrderID(orderID)
}

func (s *paymentService) GetPaymentsByUserID(userID string, limit, offset int) ([]model.Payment, int64, error) {
	return s.paymentRepo.FindByUserID(userID, limit, offset)
}

func (s *paymentService) CheckPaymentStatusFromMidtrans(orderID string) error {
	payment, err := s.paymentRepo.FindByOrderID(orderID)
	if err != nil {
		return fmt.Errorf("payment not found: %v", err)
	}
	if payment.Status == model.PaymentStatusSuccess {
		return nil
	}
	if payment.MidtransTransactionID == "" {
		return fmt.Errorf("no Midtrans transaction ID")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(s.cfg.MidtransServerKey + ":"))
	url := fmt.Sprintf("%s/%s/status", s.getBaseURL(), payment.MidtransTransactionID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Midtrans API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Midtrans API error: %s", string(body))
	}

	return s.updatePaymentFromMidtransResponse(payment, body)
}

func (s *paymentService) HandleWebhook(payload []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("invalid webhook payload: %v", err)
	}

	orderID, _ := data["order_id"].(string)
	if orderID == "" {
		return fmt.Errorf("missing order_id in webhook")
	}

	payment, err := s.paymentRepo.FindByOrderID(orderID)
	if err != nil {
		return fmt.Errorf("payment not found: %v", err)
	}

	return s.updatePaymentFromMidtransResponse(payment, payload)
}

func (s *paymentService) updatePaymentFromMidtransResponse(payment *model.Payment, body []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("invalid response: %v", err)
	}

	transactionStatus, _ := data["transaction_status"].(string)
	transactionID, _ := data["transaction_id"].(string)

	var vaNumber, bankType, qrCodeURL string
	if vaNumbers, ok := data["va_numbers"].([]interface{}); ok && len(vaNumbers) > 0 {
		if va, ok := vaNumbers[0].(map[string]interface{}); ok {
			vaNumber, _ = va["va_number"].(string)
			bankType, _ = va["bank"].(string)
		}
	}

	if actions, ok := data["actions"].([]interface{}); ok {
		for _, a := range actions {
			if act, ok := a.(map[string]interface{}); ok {
				name, _ := act["name"].(string)
				url, _ := act["url"].(string)
				if (strings.Contains(name, "qr") || name == "generate-qr-code") && url != "" {
					qrCodeURL = url
					break
				}
			}
		}
		if qrCodeURL == "" {
			for _, a := range actions {
				if act, ok := a.(map[string]interface{}); ok {
					method, _ := act["method"].(string)
					url, _ := act["url"].(string)
					if method == "GET" && url != "" {
						qrCodeURL = url
						break
					}
				}
			}
		}
	}
	if qrCodeURL == "" && payment.QRCodeURL != "" {
		qrCodeURL = payment.QRCodeURL
	}
	if vaNumber == "" {
		vaNumber = payment.VANumber
	}
	if bankType == "" {
		bankType = payment.BankType
	}

	var expiryTime *time.Time
	if expiry, ok := data["expiry_time"].(string); ok && expiry != "" {
		formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
		for _, f := range formats {
			if exp, err := time.Parse(f, expiry); err == nil {
				expiryTime = &exp
				break
			}
		}
	}

	updates := map[string]interface{}{
		"status":                  mapMidtransStatus(transactionStatus),
		"midtrans_transaction_id": transactionID,
		"va_number":               vaNumber,
		"bank_type":               bankType,
		"qr_code_url":             qrCodeURL,
		"expiry_time":             expiryTime,
		"midtrans_response":       string(body),
		"updated_at":              time.Now(),
	}

	if err := s.paymentRepo.Updates(payment.OrderID, updates); err != nil {
		return err
	}

	updatedPayment, _ := s.paymentRepo.FindByOrderID(payment.OrderID)

	// Upgrade user role jika payment berhasil dan ada target_role
	paymentStatus := mapMidtransStatus(transactionStatus)
	if paymentStatus == model.PaymentStatusSuccess && updatedPayment.TargetRole != "" {
		if err := s.userRepo.UpdateUserRole(updatedPayment.UserID, updatedPayment.TargetRole); err != nil {
			log.Printf("Failed to upgrade user role after payment: %v", err)
		} else {
			log.Printf("User %s upgraded to role %s after payment success", updatedPayment.UserID, updatedPayment.TargetRole)
		}
	}

	if s.wsHub != nil && updatedPayment.UserID != "" {
		s.wsHub.BroadcastToUser(updatedPayment.UserID, map[string]interface{}{
			"type":    "payment_status",
			"payment": updatedPayment,
		})
	}

	return nil
}

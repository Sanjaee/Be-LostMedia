package app

import (
	"io"
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService service.PaymentService
}

func NewPaymentHandler(paymentService service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

// CreatePaymentForRole handles payment for role upgrade (pakai harga dari role_prices)
// POST /api/v1/payments/role
func (h *PaymentHandler) CreatePaymentForRole(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreatePaymentForRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	payment, err := h.paymentService.CreatePaymentForRole(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Payment created successfully", gin.H{"payment": payment})
}

// CreatePayment handles payment creation
// POST /api/v1/payments
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	payment, err := h.paymentService.CreatePayment(userID.(string), req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Payment created successfully", gin.H{"payment": payment})
}

// GetPayment handles getting payment by ID
// GET /api/v1/payments/:id
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	id := c.Param("id")
	if id == "" {
		util.BadRequest(c, "Payment ID is required")
		return
	}

	payment, err := h.paymentService.GetPaymentByID(id)
	if err != nil {
		util.NotFound(c, "Payment not found")
		return
	}

	if payment.UserID != userID.(string) {
		util.Forbidden(c, "Access denied")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Payment retrieved successfully", gin.H{"payment": payment})
}

// GetPaymentByOrderID handles getting payment by order ID
// GET /api/v1/payments/order/:orderID
func (h *PaymentHandler) GetPaymentByOrderID(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	orderID := c.Param("orderID")
	if orderID == "" {
		util.BadRequest(c, "Order ID is required")
		return
	}

	payment, err := h.paymentService.GetPaymentByOrderID(orderID)
	if err != nil {
		util.NotFound(c, "Payment not found")
		return
	}

	if payment.UserID != userID.(string) {
		util.Forbidden(c, "Access denied")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Payment retrieved successfully", gin.H{"payment": payment})
}

// GetMyPayments handles getting current user's payments
// GET /api/v1/payments?limit=10&offset=0
func (h *PaymentHandler) GetMyPayments(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	payments, total, err := h.paymentService.GetPaymentsByUserID(userID.(string), limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Payments retrieved successfully", gin.H{
		"payments": payments,
		"total":    total,
	})
}

// CheckPaymentStatus checks payment status from Midtrans
// POST /api/v1/payments/:orderID/status
func (h *PaymentHandler) CheckPaymentStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	orderID := c.Param("orderID")
	if orderID == "" {
		util.BadRequest(c, "Order ID is required")
		return
	}

	payment, err := h.paymentService.GetPaymentByOrderID(orderID)
	if err != nil {
		util.NotFound(c, "Payment not found")
		return
	}

	if payment.UserID != userID.(string) {
		util.Forbidden(c, "Access denied")
		return
	}

	if err := h.paymentService.CheckPaymentStatusFromMidtrans(orderID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	payment, _ = h.paymentService.GetPaymentByOrderID(orderID)
	util.SuccessResponse(c, http.StatusOK, "Status updated", gin.H{"payment": payment})
}

// HandleWebhook handles Midtrans webhook callback (no auth)
// POST /api/v1/payments/webhook
func (h *PaymentHandler) HandleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "Failed to read body", nil)
		return
	}

	if err := h.paymentService.HandleWebhook(body); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

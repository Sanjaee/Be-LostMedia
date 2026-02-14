package app

import (
	"net/http"
	"strconv"

	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type RolePriceHandler struct {
	rolePriceService service.RolePriceService
}

func NewRolePriceHandler(rolePriceService service.RolePriceService) *RolePriceHandler {
	return &RolePriceHandler{
		rolePriceService: rolePriceService,
	}
}

// ListRolePrices returns role prices (public or with inactive for admin)
// GET /api/v1/role-prices?include_inactive=false
func (h *RolePriceHandler) ListRolePrices(c *gin.Context) {
	includeInactive, _ := strconv.ParseBool(c.DefaultQuery("include_inactive", "false"))

	list, err := h.rolePriceService.List(includeInactive)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Role prices retrieved successfully", gin.H{"role_prices": list})
}

// GetRolePrice returns role price by ID
// GET /api/v1/role-prices/:id
func (h *RolePriceHandler) GetRolePrice(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.BadRequest(c, "Role price ID is required")
		return
	}

	rp, err := h.rolePriceService.GetByID(id)
	if err != nil {
		util.NotFound(c, "Role price not found")
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Role price retrieved successfully", gin.H{"role_price": rp})
}

// GetRolePriceByRole returns role price by role name
// GET /api/v1/role-prices/role/:role
func (h *RolePriceHandler) GetRolePriceByRole(c *gin.Context) {
	role := c.Param("role")
	if role == "" {
		util.BadRequest(c, "Role is required")
		return
	}

	rp, err := h.rolePriceService.GetByRole(role)
	if err != nil {
		util.NotFound(c, "Role price not found for role: "+role)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Role price retrieved successfully", gin.H{"role_price": rp})
}

// CreateRolePrice creates role price (admin only)
// POST /api/v1/admin/role-prices
func (h *RolePriceHandler) CreateRolePrice(c *gin.Context) {
	var req service.CreateRolePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	rp, err := h.rolePriceService.Create(req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Role price created successfully", gin.H{"role_price": rp})
}

// UpdateRolePrice updates role price (admin only)
// PUT /api/v1/admin/role-prices/:id
func (h *RolePriceHandler) UpdateRolePrice(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.BadRequest(c, "Role price ID is required")
		return
	}

	var req service.UpdateRolePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	rp, err := h.rolePriceService.Update(id, req)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Role price updated successfully", gin.H{"role_price": rp})
}

// DeleteRolePrice deletes role price (admin only)
// DELETE /api/v1/admin/role-prices/:id
func (h *RolePriceHandler) DeleteRolePrice(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.BadRequest(c, "Role price ID is required")
		return
	}

	if err := h.rolePriceService.Delete(id); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Role price deleted successfully", nil)
}

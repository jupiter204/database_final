package models

import (
	"time"
)

// --- Common Response Models ---

// ErrorResponse represents a common error response structure
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

// MessageResponse represents a common success message structure
type MessageResponse struct {
	Message string `json:"message" example:"operation successful"`
}

// TokenResponse represents the tokens returned after successful login or refresh
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// --- Auth Models ---

// LoginRequest represents the data structure for user authentication
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin"`
}

// RefreshRequest represents the request to refresh an access token
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// --- Equipment Models ---

// EquipmentPublicResponse represents the equipment info returned to public users
type EquipmentPublicResponse struct {
	LID             string `json:"lid"`
	AssetCode       string `json:"asset_code"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Status          string `json:"status"`
	HasActiveReport bool   `json:"has_active_report"`
}

// EquipmentDetail represents detailed information about a piece of equipment for staff/admin
type EquipmentDetail struct {
	LID           string  `json:"lid"`
	AssetCode     string  `json:"asset_code"`
	Name          string  `json:"name"`
	Category      *string `json:"category"`
	LastMaintDate string  `json:"last_maint_date"`
	MaintInterval int     `json:"maint_interval"`
	Status        string  `json:"status"`
	Location      *string `json:"location"`
}

// CreateEquipmentRequest represents the request to add a new piece of equipment
type CreateEquipmentRequest struct {
	AssetCode     string `json:"asset_code" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Category      string `json:"category"`
	LastMaintDate string `json:"last_maint_date"`
	MaintInterval int    `json:"maint_interval"`
	Location      string `json:"location"`
}

// DeleteEquipmentRequest represents the request to remove equipment
type DeleteEquipmentRequest struct {
	LID string `json:"lid" binding:"required"`
}

// --- Maintenance Models ---

// MaintenanceRequest represents a new maintenance report from a user
type MaintenanceRequest struct {
	EquipmentID  string `json:"equipment_id" binding:"required"`
	ReporterType string `json:"reporter_type" binding:"required"`
	Description  string `json:"description" binding:"required"`
}

// ResolveMaintenanceRequest represents the request to mark a record as resolved
type ResolveMaintenanceRequest struct {
	LID         string `json:"lid" binding:"required"` // 維修紀錄列表的lid
	ResolveNote string `json:"resolve_note" binding:"required"`
}

// MaintenanceRecord represents a record in the maintenance history
type MaintenanceRecord struct {
	LID           string    `json:"lid"`
	EquipmentID   string    `json:"equipment_id"`
	EquipmentName string    `json:"equipment_name"`
	AssetCode     string    `json:"asset_code"`
	ReporterType  string    `json:"reporter_type"`
	Description   string    `json:"description"`
	IsResolved    bool      `json:"is_resolved"`
	ResolveNote   string    `json:"resolve_note"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- User Models ---

// CreateUserRequest represents the request to add a new user
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Role     string `json:"role" binding:"required"` // 'admin' or 'staff'
}

// UpdateUserRequest represents the request to update user details
type UpdateUserRequest struct {
	LID      string  `json:"lid" binding:"required"`
	Password *string `json:"password"`
	Name     *string `json:"name"`
	Role     *string `json:"role"`
}

// DeleteUserRequest represents the request to remove a user
type DeleteUserRequest struct {
	LID string `json:"lid" binding:"required"`
}

// UserResponse represents the user data returned to the client
type UserResponse struct {
	LID      string `json:"lid"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

package core

import "context"

type PrincipalRole string

const (
	RoleSupplier PrincipalRole = "supplier"
	RoleWerka    PrincipalRole = "werka"
)

type Principal struct {
	Role        PrincipalRole `json:"role"`
	DisplayName string        `json:"display_name"`
	LegalName   string        `json:"legal_name,omitempty"`
	Ref         string        `json:"ref,omitempty"`
	Phone       string        `json:"phone,omitempty"`
	AvatarURL   string        `json:"avatar_url,omitempty"`
}

type Authenticator interface {
	Login(ctx context.Context, phone, code string) (Principal, error)
}

type LoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type LoginResponse struct {
	Token   string    `json:"token"`
	Profile Principal `json:"profile"`
}

type DispatchRecord struct {
	ID           string  `json:"id"`
	SupplierName string  `json:"supplier_name"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	UOM          string  `json:"uom"`
	SentQty      float64 `json:"sent_qty"`
	AcceptedQty  float64 `json:"accepted_qty"`
	Status       string  `json:"status"`
	CreatedLabel string  `json:"created_label"`
}

type SupplierItem struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	UOM       string `json:"uom"`
	Warehouse string `json:"warehouse"`
}

type CreateDispatchRequest struct {
	ItemCode string  `json:"item_code"`
	Qty      float64 `json:"qty"`
}

type ConfirmReceiptRequest struct {
	ReceiptID   string  `json:"receipt_id"`
	AcceptedQty float64 `json:"accepted_qty"`
}

type ProfileUpdateRequest struct {
	Nickname string `json:"nickname"`
}

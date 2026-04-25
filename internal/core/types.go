package core

import "context"

type PrincipalRole string

const (
	RoleSupplier PrincipalRole = "supplier"
	RoleWerka    PrincipalRole = "werka"
	RoleCustomer PrincipalRole = "customer"
	RoleAdmin    PrincipalRole = "admin"
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
	Token     string         `json:"token"`
	Profile   Principal      `json:"profile"`
	WerkaHome *WerkaHomeData `json:"werka_home,omitempty"`
}

type DispatchRecord struct {
	ID           string  `json:"id"`
	RecordType   string  `json:"record_type,omitempty"`
	SupplierRef  string  `json:"supplier_ref,omitempty"`
	SupplierName string  `json:"supplier_name"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	UOM          string  `json:"uom"`
	SentQty      float64 `json:"sent_qty"`
	AcceptedQty  float64 `json:"accepted_qty"`
	Amount       float64 `json:"amount,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Note         string  `json:"note,omitempty"`
	EventType    string  `json:"event_type,omitempty"`
	Highlight    string  `json:"highlight,omitempty"`
	Status       string  `json:"status"`
	CreatedLabel string  `json:"created_label"`
}

type NotificationComment struct {
	ID           string `json:"id"`
	AuthorLabel  string `json:"author_label"`
	Body         string `json:"body"`
	CreatedLabel string `json:"created_label"`
}

type NotificationDetail struct {
	Record   DispatchRecord        `json:"record"`
	Comments []NotificationComment `json:"comments"`
}

type SupplierItem struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	UOM       string `json:"uom"`
	Warehouse string `json:"warehouse"`
}

type SupplierDirectoryEntry struct {
	Ref   string `json:"ref"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type CustomerDirectoryEntry struct {
	Ref   string `json:"ref"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type CustomerItemOption struct {
	CustomerRef   string `json:"customer_ref"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	ItemCode      string `json:"item_code"`
	ItemName      string `json:"item_name"`
	UOM           string `json:"uom"`
	Warehouse     string `json:"warehouse"`
}

type CustomerHomeSummary struct {
	PendingCount   int `json:"pending_count"`
	ConfirmedCount int `json:"confirmed_count"`
	RejectedCount  int `json:"rejected_count"`
}

type CustomerDeliveryResponseMode string

const (
	CustomerDeliveryResponseAcceptAll        CustomerDeliveryResponseMode = "accept_all"
	CustomerDeliveryResponseAcceptPartial    CustomerDeliveryResponseMode = "accept_partial"
	CustomerDeliveryResponseRejectAll        CustomerDeliveryResponseMode = "reject_all"
	CustomerDeliveryResponseClaimAfterAccept CustomerDeliveryResponseMode = "claim_after_accept"
)

type CustomerDeliveryDetail struct {
	Record             DispatchRecord `json:"record"`
	CanApprove         bool           `json:"can_approve"`
	CanReject          bool           `json:"can_reject"`
	CanPartiallyAccept bool           `json:"can_partially_accept,omitempty"`
	CanReportClaim     bool           `json:"can_report_claim,omitempty"`
}

type SupplierHomeSummary struct {
	PendingCount   int `json:"pending_count"`
	SubmittedCount int `json:"submitted_count"`
	ReturnedCount  int `json:"returned_count"`
}

type SupplierStatusBreakdownEntry struct {
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	ReceiptCount     int     `json:"receipt_count"`
	TotalSentQty     float64 `json:"total_sent_qty"`
	TotalAcceptedQty float64 `json:"total_accepted_qty"`
	TotalReturnedQty float64 `json:"total_returned_qty"`
	UOM              string  `json:"uom"`
}

type WerkaHomeSummary struct {
	PendingCount   int `json:"pending_count"`
	ConfirmedCount int `json:"confirmed_count"`
	ReturnedCount  int `json:"returned_count"`
}

type WerkaHomeData struct {
	Summary      WerkaHomeSummary `json:"summary"`
	PendingItems []DispatchRecord `json:"pending_items"`
}

type WerkaStatusBreakdownEntry struct {
	SupplierRef      string  `json:"supplier_ref"`
	SupplierName     string  `json:"supplier_name"`
	ReceiptCount     int     `json:"receipt_count"`
	TotalSentQty     float64 `json:"total_sent_qty"`
	TotalAcceptedQty float64 `json:"total_accepted_qty"`
	TotalReturnedQty float64 `json:"total_returned_qty"`
	UOM              string  `json:"uom"`
}

type ArchiveTotalByUOM struct {
	UOM string  `json:"uom"`
	Qty float64 `json:"qty"`
}

type WerkaArchiveSummary struct {
	RecordCount int                 `json:"record_count"`
	TotalsByUOM []ArchiveTotalByUOM `json:"totals_by_uom"`
}

type WerkaArchiveResponse struct {
	Kind    string              `json:"kind"`
	Period  string              `json:"period"`
	From    string              `json:"from,omitempty"`
	To      string              `json:"to,omitempty"`
	Summary WerkaArchiveSummary `json:"summary"`
	Items   []DispatchRecord    `json:"items"`
}

type CreateDispatchRequest struct {
	ItemCode string  `json:"item_code"`
	Qty      float64 `json:"qty"`
}

type ConfirmReceiptRequest struct {
	ReceiptID     string  `json:"receipt_id"`
	AcceptedQty   float64 `json:"accepted_qty"`
	ReturnedQty   float64 `json:"returned_qty"`
	ReturnReason  string  `json:"return_reason"`
	ReturnComment string  `json:"return_comment"`
}

type WerkaUnannouncedCreateRequest struct {
	SupplierRef string  `json:"supplier_ref"`
	ItemCode    string  `json:"item_code"`
	Qty         float64 `json:"qty"`
}

type WerkaCustomerIssueCreateRequest struct {
	CustomerRef string  `json:"customer_ref"`
	ItemCode    string  `json:"item_code"`
	Qty         float64 `json:"qty"`
}

type WerkaCustomerIssueBatchCreateRequest struct {
	ClientBatchID string                            `json:"client_batch_id"`
	Lines         []WerkaCustomerIssueCreateRequest `json:"lines"`
}

type WerkaCustomerIssueRecord struct {
	EntryID      string  `json:"entry_id"`
	CustomerRef  string  `json:"customer_ref"`
	CustomerName string  `json:"customer_name"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	UOM          string  `json:"uom"`
	Qty          float64 `json:"qty"`
	CreatedLabel string  `json:"created_label"`
}

type WerkaCustomerIssueBatchLineResult struct {
	LineIndex int                       `json:"line_index"`
	Record    *WerkaCustomerIssueRecord `json:"record,omitempty"`
	Error     string                    `json:"error,omitempty"`
	ErrorCode string                    `json:"error_code,omitempty"`
}

type WerkaCustomerIssueBatchResult struct {
	ClientBatchID string                              `json:"client_batch_id"`
	Created       []WerkaCustomerIssueBatchLineResult `json:"created"`
	Failed        []WerkaCustomerIssueBatchLineResult `json:"failed"`
}

type SupplierUnannouncedResponseRequest struct {
	ReceiptID string `json:"receipt_id"`
	Approve   bool   `json:"approve"`
	Reason    string `json:"reason"`
}

type CustomerDeliveryResponseRequest struct {
	DeliveryNoteID string                       `json:"delivery_note_id"`
	Approve        *bool                        `json:"approve,omitempty"`
	Mode           CustomerDeliveryResponseMode `json:"mode,omitempty"`
	AcceptedQty    float64                      `json:"accepted_qty,omitempty"`
	ReturnedQty    float64                      `json:"returned_qty,omitempty"`
	Reason         string                       `json:"reason,omitempty"`
	Comment        string                       `json:"comment,omitempty"`
}

type NotificationCommentCreateRequest struct {
	Message string `json:"message"`
}

type PushTokenRegisterRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

type ProfileUpdateRequest struct {
	Nickname string `json:"nickname"`
}

type AdminSettings struct {
	ERPURL                 string `json:"erp_url"`
	ERPAPIKey              string `json:"erp_api_key"`
	ERPAPISecret           string `json:"erp_api_secret"`
	DefaultTargetWarehouse string `json:"default_target_warehouse"`
	DefaultUOM             string `json:"default_uom"`
	WerkaPhone             string `json:"werka_phone"`
	WerkaName              string `json:"werka_name"`
	WerkaCode              string `json:"werka_code"`
	WerkaCodeLocked        bool   `json:"werka_code_locked"`
	WerkaCodeRetryAfterSec int    `json:"werka_code_retry_after_sec"`
	AdminPhone             string `json:"admin_phone"`
	AdminName              string `json:"admin_name"`
}

type AdminSupplier struct {
	Ref               string   `json:"ref"`
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	Code              string   `json:"code"`
	Blocked           bool     `json:"blocked"`
	Removed           bool     `json:"removed"`
	AssignedItemCodes []string `json:"assigned_item_codes"`
	AssignedItemCount int      `json:"assigned_item_count"`
}

type AdminCreateSupplierRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type AdminCreateCustomerRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type AdminSupplierSummary struct {
	TotalSuppliers   int `json:"total_suppliers"`
	ActiveSuppliers  int `json:"active_suppliers"`
	BlockedSuppliers int `json:"blocked_suppliers"`
}

type AdminSupplierDetail struct {
	Ref               string         `json:"ref"`
	Name              string         `json:"name"`
	Phone             string         `json:"phone"`
	Code              string         `json:"code"`
	Blocked           bool           `json:"blocked"`
	Removed           bool           `json:"removed"`
	CodeLocked        bool           `json:"code_locked"`
	CodeRetryAfterSec int            `json:"code_retry_after_sec"`
	AssignedItems     []SupplierItem `json:"assigned_items"`
}

type AdminCustomerDetail struct {
	Ref               string         `json:"ref"`
	Name              string         `json:"name"`
	Phone             string         `json:"phone"`
	Code              string         `json:"code"`
	CodeLocked        bool           `json:"code_locked"`
	CodeRetryAfterSec int            `json:"code_retry_after_sec"`
	AssignedItems     []SupplierItem `json:"assigned_items"`
}

type AdminCustomerPhoneUpdateRequest struct {
	Phone string `json:"phone"`
}

type AdminSupplierStatusUpdateRequest struct {
	Blocked bool `json:"blocked"`
}

type AdminSupplierPhoneUpdateRequest struct {
	Phone string `json:"phone"`
}

type AdminSupplierItemsUpdateRequest struct {
	ItemCodes []string `json:"item_codes"`
}

type AdminSupplierItemMutationRequest struct {
	ItemCode string `json:"item_code"`
}

type AdminCreateItemRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
	UOM  string `json:"uom"`
}

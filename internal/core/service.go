package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
	ErrUnauthorized       = errors.New("unauthorized")
)

type ERPClient interface {
	SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	GetSupplier(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error)
	SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]erpnext.Item, error)
	ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error)
	ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty float64) (erpnext.PurchaseReceiptSubmissionResult, error)
	UploadSupplierImage(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error)
}

type ERPAuthenticator struct {
	erp              ERPClient
	baseURL          string
	apiKey           string
	apiSecret        string
	defaultWarehouse string
	supplierPrefix   string
	werkaPrefix      string
	werkaCode        string
	werkaPhone       string
	werkaName        string
	profiles         *ProfileStore
}

func NewERPAuthenticator(
	erp ERPClient,
	baseURL string,
	apiKey string,
	apiSecret string,
	defaultWarehouse string,
	supplierPrefix string,
	werkaPrefix string,
	werkaCode string,
	werkaPhone string,
	werkaName string,
	profiles *ProfileStore,
) *ERPAuthenticator {
	if strings.TrimSpace(supplierPrefix) == "" {
		supplierPrefix = "10"
	}
	if strings.TrimSpace(werkaPrefix) == "" {
		werkaPrefix = "20"
	}
	if strings.TrimSpace(werkaName) == "" {
		werkaName = "Werka"
	}

	return &ERPAuthenticator{
		erp:              erp,
		baseURL:          strings.TrimSpace(baseURL),
		apiKey:           strings.TrimSpace(apiKey),
		apiSecret:        strings.TrimSpace(apiSecret),
		defaultWarehouse: strings.TrimSpace(defaultWarehouse),
		supplierPrefix:   strings.TrimSpace(supplierPrefix),
		werkaPrefix:      strings.TrimSpace(werkaPrefix),
		werkaCode:        strings.TrimSpace(werkaCode),
		werkaPhone:       strings.TrimSpace(werkaPhone),
		werkaName:        strings.TrimSpace(werkaName),
		profiles:         profiles,
	}
}

func (a *ERPAuthenticator) Login(ctx context.Context, phone, code string) (Principal, error) {
	role, err := a.inferRole(code)
	if err != nil {
		return Principal{}, err
	}

	normalizedPhone, err := suplier.NormalizePhone(phone)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}

	switch role {
	case RoleSupplier:
		suppliers, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
		if err != nil {
			return Principal{}, err
		}
		for _, item := range suppliers {
			creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
				Ref:   item.ID,
				Name:  item.Name,
				Phone: item.Phone,
			})
			if err != nil {
				continue
			}
			if strings.TrimSpace(code) == creds.Code &&
				strings.TrimSpace(item.Phone) != "" &&
				strings.EqualFold(strings.TrimSpace(item.Phone), normalizedPhone) {
				principal := Principal{
					Role:        RoleSupplier,
					DisplayName: item.Name,
					LegalName:   item.Name,
					Ref:         item.ID,
					Phone:       item.Phone,
				}
				return a.mergeProfilePrefs(principal), nil
			}
		}
		return Principal{}, ErrInvalidCredentials

	case RoleWerka:
		if code == a.werkaCode && code != "" {
			if a.werkaPhone != "" {
				expectedWerkaPhone, err := suplier.NormalizePhone(a.werkaPhone)
				if err != nil {
					return Principal{}, ErrInvalidCredentials
				}
				if expectedWerkaPhone != normalizedPhone {
					return Principal{}, ErrInvalidCredentials
				}
			}
			return Principal{
				Role:        RoleWerka,
				DisplayName: a.werkaName,
				LegalName:   a.werkaName,
				Ref:         "werka",
				Phone:       normalizedPhone,
			}, nil
		}
		return Principal{}, ErrInvalidCredentials

	default:
		return Principal{}, ErrInvalidRole
	}
}

func (a *ERPAuthenticator) Profile(ctx context.Context, principal Principal) (Principal, error) {
	if principal.Role == RoleSupplier {
		doc, err := a.erp.GetSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref)
		if err == nil {
			principal.Phone = doc.Phone
			if doc.Image != "" {
				principal.AvatarURL = absoluteFileURL(a.baseURL, doc.Image)
			}
		}
	}
	return a.mergeProfilePrefs(principal), nil
}

func (a *ERPAuthenticator) UpdateNickname(principal Principal, nickname string) (Principal, error) {
	if a.profiles == nil {
		return principal, nil
	}
	prefs, err := a.profiles.Get(profileKey(principal))
	if err != nil {
		return Principal{}, err
	}
	prefs.Nickname = strings.TrimSpace(nickname)
	if err := a.profiles.Put(profileKey(principal), prefs); err != nil {
		return Principal{}, err
	}
	return a.mergeProfilePrefs(principal), nil
}

func (a *ERPAuthenticator) UploadAvatar(ctx context.Context, principal Principal, filename, contentType string, content []byte) (Principal, error) {
	if principal.Role != RoleSupplier {
		return principal, nil
	}
	fileURL, err := a.erp.UploadSupplierImage(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, filename, contentType, content)
	if err != nil {
		return Principal{}, err
	}
	principal.AvatarURL = absoluteFileURL(a.baseURL, fileURL)

	if a.profiles != nil {
		prefs, err := a.profiles.Get(profileKey(principal))
		if err != nil {
			return Principal{}, err
		}
		prefs.AvatarURL = principal.AvatarURL
		if err := a.profiles.Put(profileKey(principal), prefs); err != nil {
			return Principal{}, err
		}
	}

	return a.mergeProfilePrefs(principal), nil
}

func (a *ERPAuthenticator) inferRole(code string) (PrincipalRole, error) {
	trimmed := strings.TrimSpace(code)
	switch {
	case strings.HasPrefix(trimmed, a.supplierPrefix):
		return RoleSupplier, nil
	case strings.HasPrefix(trimmed, a.werkaPrefix):
		return RoleWerka, nil
	default:
		return "", ErrInvalidRole
	}
}

func (a *ERPAuthenticator) SupplierHistory(ctx context.Context, principal Principal, limit int) ([]DispatchRecord, error) {
	items, err := a.erp.ListSupplierPurchaseReceipts(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, limit)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		sentQty := item.Qty
		if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(item.SupplierDeliveryNote); ok && markerQty > sentQty {
			sentQty = markerQty
		}
		status, acceptedQty := mapDispatchStatus(item, sentQty)
		result = append(result, DispatchRecord{
			ID:           item.Name,
			SupplierName: principal.DisplayName,
			ItemCode:     item.ItemCode,
			ItemName:     item.ItemName,
			UOM:          item.UOM,
			SentQty:      sentQty,
			AcceptedQty:  acceptedQty,
			Status:       status,
			CreatedLabel: item.PostingDate,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaPending(ctx context.Context, limit int) ([]DispatchRecord, error) {
	items, err := a.erp.ListPendingPurchaseReceipts(ctx, a.baseURL, a.apiKey, a.apiSecret, limit)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		sentQty := item.Qty
		if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(item.SupplierDeliveryNote); ok && markerQty > sentQty {
			sentQty = markerQty
		}
		result = append(result, DispatchRecord{
			ID:           item.Name,
			SupplierName: item.SupplierName,
			ItemCode:     item.ItemCode,
			ItemName:     item.ItemName,
			UOM:          item.UOM,
			SentQty:      sentQty,
			AcceptedQty:  0,
			Status:       "pending",
			CreatedLabel: item.PostingDate,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) SupplierItems(ctx context.Context, principal Principal, query string, limit int) ([]SupplierItem, error) {
	items, err := a.erp.SearchSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, query, limit)
	if err != nil {
		return nil, err
	}

	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]SupplierItem, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierItem{
			Code:      item.Code,
			Name:      item.Name,
			UOM:       item.UOM,
			Warehouse: warehouse,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) CreateDispatch(ctx context.Context, principal Principal, itemCode string, qty float64) (DispatchRecord, error) {
	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return DispatchRecord{}, err
	}

	draft, err := a.erp.CreateDraftPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreatePurchaseReceiptInput{
		Supplier:      principal.Ref,
		SupplierPhone: principal.Phone,
		ItemCode:      strings.TrimSpace(itemCode),
		Qty:           qty,
		Warehouse:     warehouse,
	})
	if err != nil {
		return DispatchRecord{}, err
	}

	return DispatchRecord{
		ID:           draft.Name,
		SupplierName: principal.DisplayName,
		ItemCode:     draft.ItemCode,
		ItemName:     draft.ItemName,
		UOM:          draft.UOM,
		SentQty:      draft.Qty,
		AcceptedQty:  0,
		Status:       "pending",
		CreatedLabel: draft.PostingDate,
	}, nil
}

func (a *ERPAuthenticator) resolveWarehouse(ctx context.Context) (string, error) {
	if strings.TrimSpace(a.defaultWarehouse) != "" {
		return strings.TrimSpace(a.defaultWarehouse), nil
	}

	items, err := a.erp.SearchWarehouses(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 1)
	if err != nil {
		return "", err
	}
	if len(items) == 0 || strings.TrimSpace(items[0].Name) == "" {
		return "", fmt.Errorf("warehouse is not configured")
	}
	return strings.TrimSpace(items[0].Name), nil
}

func (a *ERPAuthenticator) ConfirmReceipt(ctx context.Context, receiptID string, acceptedQty float64) (DispatchRecord, error) {
	result, err := a.erp.ConfirmAndSubmitPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(receiptID), acceptedQty)
	if err != nil {
		return DispatchRecord{}, err
	}

	return DispatchRecord{
		ID:           result.Name,
		SupplierName: result.Supplier,
		ItemCode:     result.ItemCode,
		ItemName:     result.ItemCode,
		UOM:          result.UOM,
		SentQty:      result.SentQty,
		AcceptedQty:  result.AcceptedQty,
		Status:       dispatchStatusFromQuantities(result.SentQty, result.AcceptedQty),
		CreatedLabel: result.Name,
	}, nil
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Principal
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Principal),
	}
}

func (m *SessionManager) Create(principal Principal) (string, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[token] = principal
	return token, nil
}

func (m *SessionManager) Get(token string) (Principal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	principal, ok := m.sessions[token]
	return principal, ok
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *SessionManager) Update(token string, principal Principal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[token]; !ok {
		return
	}
	m.sessions[token] = principal
}

func RequireRole(principal Principal, role PrincipalRole) error {
	if principal.Role != role {
		return fmt.Errorf("role %s required", role)
	}
	return nil
}

func (a *ERPAuthenticator) mergeProfilePrefs(principal Principal) Principal {
	if a.profiles == nil {
		return principal
	}
	prefs, err := a.profiles.Get(profileKey(principal))
	if err != nil {
		return principal
	}
	if strings.TrimSpace(prefs.Nickname) != "" {
		principal.DisplayName = strings.TrimSpace(prefs.Nickname)
	}
	if strings.TrimSpace(prefs.AvatarURL) != "" {
		principal.AvatarURL = strings.TrimSpace(prefs.AvatarURL)
	}
	if principal.DisplayName == "" {
		principal.DisplayName = principal.LegalName
	}
	return principal
}

func profileKey(principal Principal) string {
	return string(principal.Role) + ":" + strings.TrimSpace(principal.Ref)
}

func absoluteFileURL(baseURL, fileURL string) string {
	trimmed := strings.TrimSpace(fileURL)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return strings.TrimRight(baseURL, "/") + trimmed
}

func bytesReader(content []byte) *bytes.Reader {
	return bytes.NewReader(content)
}

func mapDispatchStatus(item erpnext.PurchaseReceiptDraft, sentQty float64) (string, float64) {
	if item.DocStatus == 2 || strings.EqualFold(strings.TrimSpace(item.Status), "Cancelled") {
		return "cancelled", 0
	}
	if item.DocStatus == 1 {
		return dispatchStatusFromQuantities(sentQty, item.Qty), item.Qty
	}
	if strings.EqualFold(strings.TrimSpace(item.Status), "Draft") {
		return "draft", 0
	}
	return "pending", 0
}

func dispatchStatusFromQuantities(sentQty, acceptedQty float64) string {
	switch {
	case acceptedQty <= 0:
		return "rejected"
	case sentQty > 0 && acceptedQty < sentQty:
		return "partial"
	default:
		return "accepted"
	}
}

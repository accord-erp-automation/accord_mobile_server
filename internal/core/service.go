package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
	ErrUnauthorized       = errors.New("unauthorized")
	htmlTagPattern        = regexp.MustCompile(`<[^>]+>`)
)

const supplierAckEventPrefix = "supplier_ack:"

type ERPClient interface {
	SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error)
	SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error)
	GetCustomer(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Customer, error)
	EnsureCustomer(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateCustomerInput) (erpnext.Customer, error)
	UpdateCustomerDetails(ctx context.Context, baseURL, apiKey, apiSecret, id, details string) error
	UpdateCustomerContact(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error
	SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	GetSupplier(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error)
	UpdateSupplierDetails(ctx context.Context, baseURL, apiKey, apiSecret, id, details string) error
	UpdateSupplierContact(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error
	GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error)
	CreateItem(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateItemInput) (erpnext.Item, error)
	EnsureSupplier(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateSupplierInput) (erpnext.Supplier, error)
	SearchCompanies(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error)
	SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]erpnext.Item, error)
	ListAssignedSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error)
	AssignSupplierToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error
	RemoveSupplierFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error
	ListCustomerItems(ctx context.Context, baseURL, apiKey, apiSecret, customerRef, query string, limit int) ([]erpnext.Item, error)
	ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	ListPendingPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error)
	ListTelegramPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	ListTelegramPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error)
	ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.PurchaseReceiptDraft, error)
	ListSupplierPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error)
	GetPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.PurchaseReceiptDraft, error)
	ListPurchaseReceiptComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error)
	ListPurchaseReceiptCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]erpnext.Comment, error)
	AddPurchaseReceiptComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error
	UpdatePurchaseReceiptRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error
	CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error)
	CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error)
	CreateAndSubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error)
	ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason, returnComment string) (erpnext.PurchaseReceiptSubmissionResult, error)
	UploadSupplierImage(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error)
	DownloadFile(ctx context.Context, baseURL, apiKey, apiSecret, fileURL string) (string, []byte, error)
}

type ERPAuthenticator struct {
	erp               ERPClient
	baseURL           string
	apiKey            string
	apiSecret         string
	defaultWarehouse  string
	supplierPrefix    string
	werkaPrefix       string
	werkaCode         string
	werkaPhone        string
	werkaName         string
	adminPhone        string
	adminName         string
	adminCode         string
	profiles          *ProfileStore
	supplierAdmin     *AdminSupplierStore
	envPersister      EnvPersister
	warehouseMu       sync.RWMutex
	resolvedWarehouse string
	companyMu         sync.RWMutex
	resolvedCompany   string
}

func (a *ERPAuthenticator) BaseURL() string {
	return a.baseURL
}

func (a *ERPAuthenticator) APIKey() string {
	return a.apiKey
}

func (a *ERPAuthenticator) APISecret() string {
	return a.apiSecret
}

func (a *ERPAuthenticator) DownloadFile(ctx context.Context, baseURL, apiKey, apiSecret, fileURL string) (string, []byte, error) {
	return a.erp.DownloadFile(ctx, baseURL, apiKey, apiSecret, fileURL)
}

type EnvPersister interface {
	Upsert(values map[string]string) error
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
	supplierAdmin *AdminSupplierStore,
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
		supplierAdmin:    supplierAdmin,
	}
}

func (a *ERPAuthenticator) SetAdminIdentity(phone, name, code string, envPersister EnvPersister) {
	normalizedPhone, err := suplier.NormalizePhone(phone)
	if err == nil {
		a.adminPhone = normalizedPhone
	} else {
		a.adminPhone = strings.TrimSpace(phone)
	}
	a.adminName = strings.TrimSpace(name)
	a.adminCode = strings.TrimSpace(code)
	a.envPersister = envPersister
}

func (a *ERPAuthenticator) Login(ctx context.Context, phone, code string) (Principal, error) {
	normalizedPhone, err := suplier.NormalizePhone(phone)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}

	if strings.TrimSpace(a.adminPhone) != "" &&
		strings.EqualFold(strings.TrimSpace(a.adminPhone), normalizedPhone) &&
		strings.TrimSpace(a.adminCode) != "" &&
		strings.TrimSpace(code) == strings.TrimSpace(a.adminCode) {
		name := strings.TrimSpace(a.adminName)
		if name == "" {
			name = "Admin"
		}
		return Principal{
			Role:        RoleAdmin,
			DisplayName: name,
			LegalName:   name,
			Ref:         "admin",
			Phone:       normalizedPhone,
		}, nil
	}

	role, err := a.inferRole(code)
	if err != nil {
		return Principal{}, err
	}

	switch role {
	case RoleSupplier:
		suppliers, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, normalizedPhone, 50)
		if err != nil {
			return Principal{}, err
		}
		if len(suppliers) == 0 {
			suppliers, err = a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
			if err != nil {
				return Principal{}, err
			}
		}
		states, err := a.adminSupplierStates()
		if err != nil {
			return Principal{}, err
		}
		for _, item := range suppliers {
			state := states[strings.TrimSpace(item.ID)]
			if state.Removed || state.Blocked {
				continue
			}
			codeValue, err := a.supplierAccessCode(item, state)
			if err != nil {
				continue
			}
			if strings.TrimSpace(code) == codeValue &&
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
	items = uniquePurchaseReceiptsByName(items)

	commentsByReceipt, err := a.purchaseReceiptCommentsByName(ctx, items, 100)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, principal.DisplayName)
		for _, comment := range commentsByReceipt[item.Name] {
			if !isSupplierAcknowledgmentComment(comment.Content) {
				continue
			}
			if !strings.Contains(record.Note, "Supplier tasdiqladi:") {
				if strings.TrimSpace(record.Note) != "" {
					record.Note += "\n"
				}
				record.Note += "Supplier tasdiqladi: Tasdiqlayman, shu holat bo‘lganini ko‘rdim."
			}
			break
		}
		result = append(result, record)
	}
	return result, nil
}

func (a *ERPAuthenticator) SupplierSummary(ctx context.Context, principal Principal) (SupplierHomeSummary, error) {
	items, err := a.collectSupplierPurchaseReceipts(ctx, principal.Ref)
	if err != nil {
		return SupplierHomeSummary{}, err
	}

	var summary SupplierHomeSummary
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, principal.DisplayName)
		switch record.Status {
		case "pending", "draft":
			summary.PendingCount++
		case "accepted":
			summary.SubmittedCount++
		case "partial", "rejected", "cancelled":
			summary.ReturnedCount++
		}
	}
	return summary, nil
}

func (a *ERPAuthenticator) SupplierStatusBreakdown(ctx context.Context, principal Principal, kind string) ([]SupplierStatusBreakdownEntry, error) {
	items, err := a.collectSupplierPurchaseReceipts(ctx, principal.Ref)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]*SupplierStatusBreakdownEntry)
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, principal.DisplayName)
		if !recordMatchesSupplierBreakdown(record, kind) {
			continue
		}
		key := strings.TrimSpace(record.ItemCode)
		if key == "" {
			key = strings.TrimSpace(record.ItemName)
		}
		entry := grouped[key]
		if entry == nil {
			entry = &SupplierStatusBreakdownEntry{
				ItemCode: record.ItemCode,
				ItemName: record.ItemName,
				UOM:      record.UOM,
			}
			grouped[key] = entry
		}
		entry.ReceiptCount++
		entry.TotalSentQty += record.SentQty
		entry.TotalAcceptedQty += record.AcceptedQty
		entry.TotalReturnedQty += maxFloat(record.SentQty-record.AcceptedQty, 0)
		if entry.UOM == "" {
			entry.UOM = record.UOM
		}
	}

	result := make([]SupplierStatusBreakdownEntry, 0, len(grouped))
	for _, entry := range grouped {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ReceiptCount != result[j].ReceiptCount {
			return result[i].ReceiptCount > result[j].ReceiptCount
		}
		return strings.ToLower(result[i].ItemName) < strings.ToLower(result[j].ItemName)
	})
	return result, nil
}

func (a *ERPAuthenticator) SupplierStatusDetails(ctx context.Context, principal Principal, kind, itemCode string) ([]DispatchRecord, error) {
	items, err := a.collectSupplierPurchaseReceipts(ctx, principal.Ref)
	if err != nil {
		return nil, err
	}

	needle := strings.TrimSpace(itemCode)
	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, principal.DisplayName)
		if !recordMatchesSupplierBreakdown(record, kind) {
			continue
		}
		if needle != "" && !strings.EqualFold(strings.TrimSpace(record.ItemCode), needle) {
			continue
		}
		result = append(result, record)
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
		if shouldHideStaleWerkaDraft(item) {
			continue
		}
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if record.Status != "pending" && record.Status != "draft" {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaSummary(ctx context.Context) (WerkaHomeSummary, error) {
	items, err := a.collectTelegramPurchaseReceipts(ctx)
	if err != nil {
		return WerkaHomeSummary{}, err
	}

	var summary WerkaHomeSummary
	for _, item := range items {
		if shouldHideStaleWerkaDraft(item) {
			continue
		}
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if record.EventType != "" {
			continue
		}
		switch record.Status {
		case "pending", "draft":
			summary.PendingCount++
		case "accepted":
			summary.ConfirmedCount++
		case "partial", "rejected", "cancelled":
			summary.ReturnedCount++
		}
	}
	return summary, nil
}

func (a *ERPAuthenticator) WerkaStatusBreakdown(ctx context.Context, kind string) ([]WerkaStatusBreakdownEntry, error) {
	items, err := a.collectTelegramPurchaseReceipts(ctx)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]*WerkaStatusBreakdownEntry)
	for _, item := range items {
		if shouldHideStaleWerkaDraft(item) {
			continue
		}
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if record.EventType != "" {
			continue
		}
		if !recordMatchesWerkaBreakdown(record, kind) {
			continue
		}

		key := strings.TrimSpace(record.SupplierRef)
		if key == "" {
			key = strings.TrimSpace(record.SupplierName)
		}
		entry := grouped[key]
		if entry == nil {
			entry = &WerkaStatusBreakdownEntry{
				SupplierRef:  record.SupplierRef,
				SupplierName: record.SupplierName,
				UOM:          record.UOM,
			}
			grouped[key] = entry
		}
		entry.ReceiptCount++
		entry.TotalSentQty += record.SentQty
		entry.TotalAcceptedQty += record.AcceptedQty
		entry.TotalReturnedQty += maxFloat(record.SentQty-record.AcceptedQty, 0)
		if entry.UOM == "" {
			entry.UOM = record.UOM
		}
	}

	result := make([]WerkaStatusBreakdownEntry, 0, len(grouped))
	for _, entry := range grouped {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ReceiptCount != result[j].ReceiptCount {
			return result[i].ReceiptCount > result[j].ReceiptCount
		}
		return strings.ToLower(result[i].SupplierName) < strings.ToLower(result[j].SupplierName)
	})
	return result, nil
}

func (a *ERPAuthenticator) WerkaStatusDetails(ctx context.Context, kind, supplierRef string) ([]DispatchRecord, error) {
	items, err := a.collectTelegramPurchaseReceipts(ctx)
	if err != nil {
		return nil, err
	}

	needle := strings.TrimSpace(supplierRef)
	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		if shouldHideStaleWerkaDraft(item) {
			continue
		}
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if record.EventType != "" {
			continue
		}
		if needle != "" && !strings.EqualFold(strings.TrimSpace(record.SupplierRef), needle) {
			continue
		}
		if !recordMatchesWerkaBreakdown(record, kind) {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaHistory(ctx context.Context, limit int) ([]DispatchRecord, error) {
	items, err := a.erp.ListTelegramPurchaseReceipts(ctx, a.baseURL, a.apiKey, a.apiSecret, limit)
	if err != nil {
		return nil, err
	}
	items = uniquePurchaseReceiptsByName(items)

	commentsByReceipt, err := a.purchaseReceiptCommentsByName(ctx, items, 100)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if record.EventType == "werka_unannounced_pending" {
			continue
		}
		result = append(result, record)

		for _, comment := range commentsByReceipt[item.Name] {
			if !isSupplierAcknowledgmentComment(comment.Content) {
				continue
			}
			result = append(result, DispatchRecord{
				ID:           supplierAckEventPrefix + item.Name + ":" + comment.ID,
				SupplierRef:  item.Supplier,
				SupplierName: item.SupplierName,
				ItemCode:     item.ItemCode,
				ItemName:     item.ItemName,
				UOM:          item.UOM,
				SentQty:      record.SentQty,
				AcceptedQty:  record.AcceptedQty,
				Note:         "",
				EventType:    "supplier_ack",
				Highlight:    "Supplier mahsulotni qaytarganingizni tasdiqladi",
				Status:       "accepted",
				CreatedLabel: comment.CreatedAt,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	return result, nil
}

func uniquePurchaseReceiptsByName(items []erpnext.PurchaseReceiptDraft) []erpnext.PurchaseReceiptDraft {
	if len(items) < 2 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]erpnext.PurchaseReceiptDraft, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, item)
	}
	return result
}

func (a *ERPAuthenticator) purchaseReceiptCommentsByName(ctx context.Context, items []erpnext.PurchaseReceiptDraft, limit int) (map[string][]erpnext.Comment, error) {
	if len(items) == 0 {
		return map[string][]erpnext.Comment{}, nil
	}

	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		record := mapPurchaseReceiptToDispatchRecord(item, item.SupplierName)
		if !dispatchRecordNeedsCommentScan(record) {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return map[string][]erpnext.Comment{}, nil
	}

	return a.erp.ListPurchaseReceiptCommentsBatch(ctx, a.baseURL, a.apiKey, a.apiSecret, names, limit)
}

func (a *ERPAuthenticator) collectSupplierPurchaseReceipts(ctx context.Context, supplierRef string) ([]erpnext.PurchaseReceiptDraft, error) {
	const pageSize = 200
	result := make([]erpnext.PurchaseReceiptDraft, 0, pageSize)
	seen := make(map[string]struct{}, pageSize)
	for offset := 0; ; offset += pageSize {
		items, err := a.erp.ListSupplierPurchaseReceiptsPage(ctx, a.baseURL, a.apiKey, a.apiSecret, supplierRef, pageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, item)
		}
		if len(items) < pageSize {
			return result, nil
		}
	}
}

func (a *ERPAuthenticator) collectTelegramPurchaseReceipts(ctx context.Context) ([]erpnext.PurchaseReceiptDraft, error) {
	const pageSize = 200
	result := make([]erpnext.PurchaseReceiptDraft, 0, pageSize)
	seen := make(map[string]struct{}, pageSize)
	for offset := 0; ; offset += pageSize {
		items, err := a.erp.ListTelegramPurchaseReceiptsPage(ctx, a.baseURL, a.apiKey, a.apiSecret, pageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, item)
		}
		if len(items) < pageSize {
			return result, nil
		}
	}
}

func dispatchRecordNeedsCommentScan(record DispatchRecord) bool {
	switch record.Status {
	case "partial", "rejected", "cancelled":
		return true
	}
	return strings.TrimSpace(record.Note) != ""
}

func recordMatchesWerkaBreakdown(record DispatchRecord, kind string) bool {
	switch strings.TrimSpace(kind) {
	case "pending":
		return record.Status == "pending" || record.Status == "draft"
	case "confirmed":
		return record.Status == "accepted"
	case "returned":
		return record.Status == "partial" || record.Status == "rejected" || record.Status == "cancelled"
	default:
		return false
	}
}

func recordMatchesSupplierBreakdown(record DispatchRecord, kind string) bool {
	switch strings.TrimSpace(kind) {
	case "pending":
		return record.Status == "pending" || record.Status == "draft"
	case "submitted":
		return record.Status == "accepted"
	case "returned":
		return record.Status == "partial" || record.Status == "rejected" || record.Status == "cancelled"
	default:
		return false
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func shouldHideStaleWerkaDraft(item erpnext.PurchaseReceiptDraft) bool {
	if item.DocStatus != 0 {
		return false
	}
	if strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks)) == "rejected" {
		return true
	}
	return strings.TrimSpace(erpnext.ExtractAccordDecisionNote(item.Remarks)) != ""
}

func (a *ERPAuthenticator) NotificationDetail(ctx context.Context, principal Principal, receiptID string) (NotificationDetail, error) {
	trimmedReceiptID := strings.TrimSpace(receiptID)
	eventType := ""
	if strings.HasPrefix(trimmedReceiptID, supplierAckEventPrefix) {
		eventType = "supplier_ack"
		parts := strings.SplitN(strings.TrimPrefix(trimmedReceiptID, supplierAckEventPrefix), ":", 2)
		if len(parts) > 0 {
			trimmedReceiptID = strings.TrimSpace(parts[0])
		}
	}

	draft, err := a.erp.GetPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, trimmedReceiptID)
	if err != nil {
		return NotificationDetail{}, err
	}
	if principal.Role == RoleSupplier && strings.TrimSpace(draft.Supplier) != strings.TrimSpace(principal.Ref) {
		return NotificationDetail{}, ErrUnauthorized
	}

	record := mapPurchaseReceiptToDispatchRecord(draft, draft.SupplierName)
	if principal.Role == RoleSupplier &&
		draft.DocStatus == 0 &&
		strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(draft.Remarks)) == "pending" {
		record.EventType = "werka_unannounced_pending"
		record.Highlight = "Werka siz qayd etmagan mahsulotni qabul qildi"
	}
	if eventType == "supplier_ack" {
		record.ID = strings.TrimSpace(receiptID)
		record.EventType = eventType
		record.Highlight = "Supplier mahsulotni qaytarganingizni tasdiqladi"
	}
	if principal.Role == RoleSupplier && strings.TrimSpace(principal.DisplayName) != "" {
		record.SupplierName = principal.DisplayName
	}

	comments, err := a.erp.ListPurchaseReceiptComments(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, 100)
	if err != nil {
		return NotificationDetail{}, err
	}

	result := make([]NotificationComment, 0, len(comments))
	for _, item := range comments {
		authorLabel, body := parseNotificationComment(item.Content)
		if body == "" {
			continue
		}
		result = append(result, NotificationComment{
			ID:           item.ID,
			AuthorLabel:  authorLabel,
			Body:         body,
			CreatedLabel: item.CreatedAt,
		})
	}

	return NotificationDetail{
		Record:   record,
		Comments: result,
	}, nil
}

func (a *ERPAuthenticator) AddNotificationComment(ctx context.Context, principal Principal, receiptID, message string) (NotificationDetail, error) {
	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" {
		return NotificationDetail{}, fmt.Errorf("comment is required")
	}

	detail, err := a.NotificationDetail(ctx, principal, receiptID)
	if err != nil {
		return NotificationDetail{}, err
	}

	formatted := formatNotificationComment(principal, trimmedMessage)
	if err := a.erp.AddPurchaseReceiptComment(ctx, a.baseURL, a.apiKey, a.apiSecret, detail.Record.ID, formatted); err != nil {
		return NotificationDetail{}, err
	}
	if principal.Role == RoleSupplier && isSupplierAcknowledgmentMessage(trimmedMessage) {
		draft, err := a.erp.GetPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, detail.Record.ID)
		if err != nil {
			return NotificationDetail{}, err
		}
		remarks := erpnext.UpsertSupplierAcknowledgmentInRemarks(
			draft.Remarks,
			trimmedMessage,
		)
		if err := a.erp.UpdatePurchaseReceiptRemarks(ctx, a.baseURL, a.apiKey, a.apiSecret, detail.Record.ID, remarks); err != nil {
			// Supplier acknowledgment is already stored as a comment; remarks backfill is best-effort.
		}
	}
	return a.NotificationDetail(ctx, principal, receiptID)
}

func (a *ERPAuthenticator) WerkaSuppliers(ctx context.Context, limit int) ([]SupplierDirectoryEntry, error) {
	items, err := a.AdminSuppliers(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]SupplierDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierDirectoryEntry{
			Ref:   item.Ref,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaCustomers(ctx context.Context, query string, limit int) ([]CustomerDirectoryEntry, error) {
	items, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, query, limit)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, CustomerDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaCustomerItems(ctx context.Context, customerRef, query string, limit int) ([]SupplierItem, error) {
	items, err := a.erp.ListCustomerItems(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(customerRef), query, limit)
	if err != nil {
		return nil, err
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) CreateWerkaCustomerIssue(ctx context.Context, principal Principal, customerRef, itemCode string, qty float64) (WerkaCustomerIssueRecord, error) {
	if principal.Role != RoleWerka {
		return WerkaCustomerIssueRecord{}, ErrUnauthorized
	}
	customer, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(customerRef))
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	items, err := a.erp.ListCustomerItems(ctx, a.baseURL, a.apiKey, a.apiSecret, customer.ID, "", 500)
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	allowed := false
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Code), strings.TrimSpace(itemCode)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return WerkaCustomerIssueRecord{}, fmt.Errorf("customer uchun mahsulot biriktirilmagan")
	}
	resolvedItems, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, []string{strings.TrimSpace(itemCode)})
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	if len(resolvedItems) == 0 {
		return WerkaCustomerIssueRecord{}, fmt.Errorf("item topilmadi")
	}
	item := resolvedItems[0]
	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	company, err := a.resolveCompany(ctx)
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	result, err := a.erp.CreateAndSubmitDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateDeliveryNoteInput{
		Customer:  customer.ID,
		Company:   company,
		Warehouse: warehouse,
		ItemCode:  strings.TrimSpace(itemCode),
		Qty:       qty,
		UOM:       item.UOM,
	})
	if err != nil {
		return WerkaCustomerIssueRecord{}, err
	}
	return WerkaCustomerIssueRecord{
		EntryID:      result.Name,
		CustomerRef:  customer.ID,
		CustomerName: customer.Name,
		ItemCode:     item.Code,
		ItemName:     item.Name,
		UOM:          item.UOM,
		Qty:          qty,
		CreatedLabel: time.Now().Format("2006-01-02"),
	}, nil
}

func (a *ERPAuthenticator) WerkaSupplierItems(ctx context.Context, supplierRef, query string, limit int) ([]SupplierItem, error) {
	principal := Principal{Role: RoleSupplier, Ref: strings.TrimSpace(supplierRef)}
	return a.supplierAllowedItems(ctx, principal, query, limit)
}

func (a *ERPAuthenticator) CreateWerkaUnannouncedDraft(ctx context.Context, principal Principal, supplierRef, itemCode string, qty float64) (DispatchRecord, error) {
	if principal.Role != RoleWerka {
		return DispatchRecord{}, ErrUnauthorized
	}
	supplier, _, err := a.findSupplierForAdmin(ctx, supplierRef)
	if err != nil {
		return DispatchRecord{}, err
	}
	if err := a.validateSupplierItemAllowed(ctx, supplier.ID, itemCode); err != nil {
		return DispatchRecord{}, err
	}
	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return DispatchRecord{}, err
	}
	draft, err := a.erp.CreateDraftPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreatePurchaseReceiptInput{
		Supplier:      supplier.ID,
		SupplierPhone: supplier.Phone,
		ItemCode:      strings.TrimSpace(itemCode),
		Qty:           qty,
		Warehouse:     warehouse,
	})
	if err != nil {
		return DispatchRecord{}, err
	}
	remarks := erpnext.UpsertWerkaUnannouncedInRemarks(draft.Remarks, "pending", "")
	if err := a.erp.UpdatePurchaseReceiptRemarks(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, remarks); err != nil {
		return DispatchRecord{}, err
	}
	_ = a.erp.AddPurchaseReceiptComment(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, formatNotificationComment(principal, "Aytilmagan mol sifatida qayd qilindi."))

	draft.Remarks = remarks
	record := mapPurchaseReceiptToDispatchRecord(draft, supplier.Name)
	record.EventType = "werka_unannounced_pending"
	record.Highlight = "Werka siz qayd etmagan mahsulotni qabul qildi"
	return record, nil
}

func (a *ERPAuthenticator) RespondWerkaUnannouncedDraft(ctx context.Context, principal Principal, receiptID string, approve bool, reason string) (NotificationDetail, error) {
	if principal.Role != RoleSupplier {
		return NotificationDetail{}, ErrUnauthorized
	}
	draft, err := a.erp.GetPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(receiptID))
	if err != nil {
		return NotificationDetail{}, err
	}
	if strings.TrimSpace(draft.Supplier) != strings.TrimSpace(principal.Ref) {
		return NotificationDetail{}, ErrUnauthorized
	}
	if strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(draft.Remarks)) != "pending" {
		return NotificationDetail{}, fmt.Errorf("unannounced draft is not pending")
	}

	if approve {
		approvedRemarks := erpnext.UpsertWerkaUnannouncedInRemarks(draft.Remarks, "approved", "")
		if err := a.erp.UpdatePurchaseReceiptRemarks(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, approvedRemarks); err != nil {
			return NotificationDetail{}, err
		}
		result, err := a.erp.ConfirmAndSubmitPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, draft.Qty, 0, "", "")
		if err != nil {
			return NotificationDetail{}, err
		}
		_ = a.erp.AddPurchaseReceiptComment(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, formatNotificationComment(principal, "Aytilmagan mol tasdiqlandi."))
		detail, err := a.NotificationDetail(ctx, principal, receiptID)
		if err != nil {
			return NotificationDetail{}, err
		}
		detail.Record.AcceptedQty = result.AcceptedQty
		detail.Record.Status = "accepted"
		detail.Record.EventType = ""
		detail.Record.Highlight = ""
		detail.Record.Note = ""
		return detail, nil
	}

	remarks := erpnext.UpsertWerkaUnannouncedInRemarks(draft.Remarks, "rejected", reason)
	if err := a.erp.UpdatePurchaseReceiptRemarks(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, remarks); err != nil {
		return NotificationDetail{}, err
	}
	message := "Aytilmagan mol rad etildi."
	if strings.TrimSpace(reason) != "" {
		message += " Sabab: " + strings.TrimSpace(reason)
	}
	_ = a.erp.AddPurchaseReceiptComment(ctx, a.baseURL, a.apiKey, a.apiSecret, draft.Name, formatNotificationComment(principal, message))
	return a.NotificationDetail(ctx, principal, receiptID)
}

func (a *ERPAuthenticator) AdminActivity(ctx context.Context, limit int) ([]DispatchRecord, error) {
	return a.WerkaHistory(ctx, limit)
}

func (a *ERPAuthenticator) SupplierItems(ctx context.Context, principal Principal, query string, limit int) ([]SupplierItem, error) {
	return a.supplierAllowedItems(ctx, principal, query, limit)
}

func (a *ERPAuthenticator) CreateDispatch(ctx context.Context, principal Principal, itemCode string, qty float64) (DispatchRecord, error) {
	if err := a.validateSupplierItemAllowed(ctx, principal.Ref, itemCode); err != nil {
		return DispatchRecord{}, err
	}

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
	if cached := a.cachedWarehouse(); cached != "" {
		return cached, nil
	}

	items, err := a.erp.SearchWarehouses(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 1)
	if err != nil {
		return "", err
	}
	if len(items) == 0 || strings.TrimSpace(items[0].Name) == "" {
		return "", fmt.Errorf("warehouse is not configured")
	}
	warehouse := strings.TrimSpace(items[0].Name)
	a.setCachedWarehouse(warehouse)
	return warehouse, nil
}

func (a *ERPAuthenticator) resolveCompany(ctx context.Context) (string, error) {
	if cached := a.cachedCompany(); cached != "" {
		return cached, nil
	}
	items, err := a.erp.SearchCompanies(ctx, a.baseURL, a.apiKey, a.apiSecret, 1)
	if err != nil {
		return "", err
	}
	if len(items) == 0 || strings.TrimSpace(items[0].Name) == "" {
		return "", fmt.Errorf("company is not configured")
	}
	company := strings.TrimSpace(items[0].Name)
	a.setCachedCompany(company)
	return company, nil
}

func (a *ERPAuthenticator) ConfirmReceipt(ctx context.Context, receiptID string, acceptedQty, returnedQty float64, returnReason, returnComment string) (DispatchRecord, error) {
	result, err := a.erp.ConfirmAndSubmitPurchaseReceipt(
		ctx,
		a.baseURL,
		a.apiKey,
		a.apiSecret,
		strings.TrimSpace(receiptID),
		acceptedQty,
		returnedQty,
		returnReason,
		returnComment,
	)
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
		Note:         result.Note,
		Status:       dispatchStatusFromQuantities(result.SentQty, result.AcceptedQty),
		CreatedLabel: result.Name,
	}, nil
}

func (a *ERPAuthenticator) AdminSettings() AdminSettings {
	werkaState, _ := a.adminSupplierState(werkaStateRef)
	return AdminSettings{
		ERPURL:                 a.baseURL,
		ERPAPIKey:              a.apiKey,
		ERPAPISecret:           a.apiSecret,
		DefaultTargetWarehouse: a.defaultWarehouse,
		DefaultUOM:             "Kg",
		WerkaPhone:             a.werkaPhone,
		WerkaName:              a.werkaName,
		WerkaCode:              a.werkaCode,
		WerkaCodeLocked:        werkaState.isCodeLocked(a.nowUTC()),
		WerkaCodeRetryAfterSec: werkaState.retryAfterSeconds(a.nowUTC()),
		AdminPhone:             a.adminPhone,
		AdminName:              a.adminName,
	}
}

func (a *ERPAuthenticator) UpdateAdminSettings(input AdminSettings) error {
	a.baseURL = strings.TrimSpace(input.ERPURL)
	a.apiKey = strings.TrimSpace(input.ERPAPIKey)
	a.apiSecret = strings.TrimSpace(input.ERPAPISecret)
	a.defaultWarehouse = strings.TrimSpace(input.DefaultTargetWarehouse)
	a.setCachedWarehouse("")
	a.werkaPhone = strings.TrimSpace(input.WerkaPhone)
	a.werkaName = strings.TrimSpace(input.WerkaName)
	a.werkaCode = strings.TrimSpace(input.WerkaCode)
	a.adminPhone = strings.TrimSpace(input.AdminPhone)
	a.adminName = strings.TrimSpace(input.AdminName)

	if a.envPersister != nil {
		return a.envPersister.Upsert(map[string]string{
			"ERP_URL":                      a.baseURL,
			"ERP_API_KEY":                  a.apiKey,
			"ERP_API_SECRET":               a.apiSecret,
			"ERP_DEFAULT_TARGET_WAREHOUSE": a.defaultWarehouse,
			"ERP_DEFAULT_UOM":              strings.TrimSpace(input.DefaultUOM),
			"WERKA_PHONE":                  a.werkaPhone,
			"WERKA_NAME":                   a.werkaName,
			"MOBILE_DEV_WERKA_CODE":        a.werkaCode,
			"ADMINKA_PHONE":                a.adminPhone,
			"ADMINKA_NAME":                 a.adminName,
		})
	}
	return nil
}

func (a *ERPAuthenticator) AdminRegenerateWerkaCode() (AdminSettings, error) {
	now := a.nowUTC()
	state, err := a.adminSupplierState(werkaStateRef)
	if err != nil {
		return AdminSettings{}, err
	}
	state, err = a.bumpCodeRegenState(state, now)
	if err != nil {
		return AdminSettings{}, err
	}

	code, err := randomSupplierCode(a.werkaPrefix, map[string]struct{}{})
	if err != nil {
		return AdminSettings{}, err
	}
	a.werkaCode = code
	state.CustomCode = code
	state.UpdatedAt = now
	if err := a.saveAdminSupplierState(werkaStateRef, state); err != nil {
		return AdminSettings{}, err
	}
	if a.envPersister != nil {
		if err := a.envPersister.Upsert(map[string]string{
			"MOBILE_DEV_WERKA_CODE": a.werkaCode,
		}); err != nil {
			return AdminSettings{}, err
		}
	}
	return a.AdminSettings(), nil
}

func (a *ERPAuthenticator) nowUTC() time.Time {
	return time.Now().UTC()
}

func (a *ERPAuthenticator) cachedWarehouse() string {
	a.warehouseMu.RLock()
	defer a.warehouseMu.RUnlock()
	return strings.TrimSpace(a.resolvedWarehouse)
}

func (a *ERPAuthenticator) setCachedWarehouse(warehouse string) {
	a.warehouseMu.Lock()
	defer a.warehouseMu.Unlock()
	a.resolvedWarehouse = strings.TrimSpace(warehouse)
}

func (a *ERPAuthenticator) cachedCompany() string {
	a.companyMu.RLock()
	defer a.companyMu.RUnlock()
	return strings.TrimSpace(a.resolvedCompany)
}

func (a *ERPAuthenticator) setCachedCompany(company string) {
	a.companyMu.Lock()
	defer a.companyMu.Unlock()
	a.resolvedCompany = strings.TrimSpace(company)
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
	if item.DocStatus == 0 {
		if strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks)) == "rejected" {
			return "cancelled", 0
		}
		acceptedFromNote, returnedFromNote := erpnext.ExtractAccordDecisionQuantities(item.Remarks)
		if acceptedFromNote <= 0 && returnedFromNote >= sentQty && returnedFromNote > 0 {
			return "cancelled", 0
		}
	}
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

func mapPurchaseReceiptToDispatchRecord(item erpnext.PurchaseReceiptDraft, fallbackSupplierName string) DispatchRecord {
	sentQty := item.Qty
	if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(item.SupplierDeliveryNote); ok && markerQty > sentQty {
		sentQty = markerQty
	}
	status, acceptedQty := mapDispatchStatus(item, sentQty)
	supplierName := strings.TrimSpace(item.SupplierName)
	if supplierName == "" {
		supplierName = strings.TrimSpace(fallbackSupplierName)
	}
	if status == "pending" {
		acceptedQty = 0
	}
	note := erpnext.ExtractAccordDecisionNote(item.Remarks)
	if note == "" &&
		item.DocStatus == 0 &&
		strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks)) == "pending" {
		note = "Werka siz qayd etmagan mahsulotni qabul qildi. Tasdiqlash kutilmoqda."
	}
	if note == "" && strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks)) == "rejected" {
		note = "Supplier aytilmagan molni rad etdi."
		if reason := strings.TrimSpace(erpnext.ExtractWerkaUnannouncedReason(item.Remarks)); reason != "" {
			note += "\nSabab: " + reason
		}
	}
	eventType := ""
	if item.DocStatus == 0 && strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks)) == "pending" {
		eventType = "werka_unannounced_pending"
	}
	return DispatchRecord{
		ID:           item.Name,
		SupplierRef:  item.Supplier,
		SupplierName: supplierName,
		ItemCode:     item.ItemCode,
		ItemName:     item.ItemName,
		UOM:          item.UOM,
		SentQty:      sentQty,
		AcceptedQty:  acceptedQty,
		Amount:       item.Amount,
		Currency:     item.Currency,
		Note:         note,
		EventType:    eventType,
		Highlight:    "",
		Status:       status,
		CreatedLabel: item.PostingDate,
	}
}

func formatNotificationComment(principal Principal, message string) string {
	label := "Tizim"
	switch principal.Role {
	case RoleSupplier:
		label = "Supplier"
	case RoleWerka:
		label = "Werka"
	case RoleAdmin:
		label = "Admin"
	}
	name := strings.TrimSpace(principal.DisplayName)
	if name == "" {
		return label + "\n" + strings.TrimSpace(message)
	}
	return label + " • " + name + "\n" + strings.TrimSpace(message)
}

func parseNotificationComment(content string) (string, string) {
	trimmed := sanitizeNotificationComment(content)
	if trimmed == "" {
		return "", ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) >= 2 {
		head := strings.TrimSpace(lines[0])
		body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
		if body != "" && (strings.HasPrefix(head, "Supplier") || strings.HasPrefix(head, "Werka") || strings.HasPrefix(head, "Admin")) {
			return head, body
		}
	}
	return "Tizim", trimmed
}

func sanitizeNotificationComment(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	replaced := strings.ReplaceAll(trimmed, "<br>", "\n")
	replaced = strings.ReplaceAll(replaced, "<br/>", "\n")
	replaced = strings.ReplaceAll(replaced, "<br />", "\n")
	replaced = htmlTagPattern.ReplaceAllString(replaced, "")
	replaced = html.UnescapeString(replaced)
	lines := strings.Split(strings.ReplaceAll(replaced, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned == "" {
			continue
		}
		filtered = append(filtered, cleaned)
	}
	return strings.Join(filtered, "\n")
}

func isSupplierAcknowledgmentMessage(message string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(message)), "tasdiqlayman")
}

func isSupplierAcknowledgmentComment(content string) bool {
	author, body := parseNotificationComment(content)
	return strings.HasPrefix(author, "Supplier") && isSupplierAcknowledgmentMessage(body)
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

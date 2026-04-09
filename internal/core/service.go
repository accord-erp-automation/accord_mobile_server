package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidInput       = errors.New("invalid input")
	htmlTagPattern        = regexp.MustCompile(`<[^>]+>`)
)

const (
	supplierAckEventPrefix            = "supplier_ack:"
	customerDeliveryResultEventPrefix = "customer_delivery_result:"
	notificationTargetPurchaseReceipt = "purchase_receipt"
	notificationTargetDeliveryNote    = "delivery_note"
	minCustomerRejectReasonRunes      = 3
	deliveryFlowStateNone             = 0
	deliveryFlowStateSubmitted        = 1
	deliveryFlowStateReturned         = 2
	customerStatePending              = 1
	customerStateRejected             = 2
	customerStateConfirmed            = 3
	customerStatePartial              = 4
	deliveryActorUnknown              = 0
	deliveryActorWerka                = 1
	customerQtyTolerance              = 0.0001
)

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
	GetItemCustomerAssignment(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (erpnext.ItemCustomerAssignment, error)
	AssignCustomerToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error
	RemoveCustomerFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error
	ListCustomerDeliveryNotes(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit int) ([]erpnext.DeliveryNoteDraft, error)
	ListCustomerDeliveryNotesPage(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit, offset int) ([]erpnext.DeliveryNoteDraft, error)
	GetDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error)
	ListDeliveryNoteComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error)
	ListDeliveryNoteCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]erpnext.Comment, error)
	AddDeliveryNoteComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error
	DeleteDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error
	CreateAndSubmitDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (erpnext.DeliveryNoteResult, error)
	CreateAndSubmitPartialDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, returnedQty float64) (erpnext.DeliveryNoteResult, error)
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
	CreateDraftDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error)
	CreateAndSubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error)
	SubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error
	EnsureDeliveryNoteStateFields(ctx context.Context, baseURL, apiKey, apiSecret string) error
	UpdateDeliveryNoteState(ctx context.Context, baseURL, apiKey, apiSecret, name string, update erpnext.DeliveryNoteStateUpdate) error
	UpdateDeliveryNoteRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error
	ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason, returnComment string) (erpnext.PurchaseReceiptSubmissionResult, error)
	UploadSupplierImage(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error)
	DownloadFile(ctx context.Context, baseURL, apiKey, apiSecret, fileURL string) (string, []byte, error)
}

type DirectoryReader interface {
	WerkaHome(ctx context.Context, pendingLimit int) (WerkaHomeData, error)
	WerkaStatusBreakdown(ctx context.Context, kind string) ([]WerkaStatusBreakdownEntry, error)
	WerkaStatusDetails(ctx context.Context, kind, supplierRef string) ([]DispatchRecord, error)
	WerkaHistory(ctx context.Context) ([]DispatchRecord, error)
	SearchWerkaSuppliersPage(ctx context.Context, query string, limit, offset int) ([]SupplierDirectoryEntry, error)
	SearchWerkaCustomersPage(ctx context.Context, query string, limit, offset int) ([]CustomerDirectoryEntry, error)
	SearchWerkaSupplierItemsPage(ctx context.Context, supplierRef, query string, limit, offset int) ([]SupplierItem, error)
	SearchWerkaCustomerItemsPage(ctx context.Context, customerRef, query string, limit, offset int) ([]SupplierItem, error)
	SearchWerkaCustomerItemOptionsPage(ctx context.Context, query string, limit, offset int) ([]CustomerItemOption, error)
	WerkaSummary(ctx context.Context) (WerkaHomeSummary, error)
	SupplierSummary(ctx context.Context, supplierRef string) (SupplierHomeSummary, error)
	CustomerSummary(ctx context.Context, customerRef string) (CustomerHomeSummary, error)
}

type ERPAuthenticator struct {
	erp               ERPClient
	reader            DirectoryReader
	baseURL           string
	apiKey            string
	apiSecret         string
	defaultWarehouse  string
	defaultUOM        string
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

func (a *ERPAuthenticator) SetDirectoryReader(reader DirectoryReader) {
	a.reader = reader
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
	defaultUOM := strings.TrimSpace(os.Getenv("ERP_DEFAULT_UOM"))
	if defaultUOM == "" {
		defaultUOM = "Kg"
	}

	return &ERPAuthenticator{
		erp:              erp,
		baseURL:          strings.TrimSpace(baseURL),
		apiKey:           strings.TrimSpace(apiKey),
		apiSecret:        strings.TrimSpace(apiSecret),
		defaultWarehouse: strings.TrimSpace(defaultWarehouse),
		defaultUOM:       defaultUOM,
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
	normalizedPhone, err := normalizeConfigPhone(phone)
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

	case RoleCustomer:
		customers, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, normalizedPhone, 50)
		if err != nil {
			return Principal{}, err
		}
		if len(customers) == 0 {
			customers, err = a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
			if err != nil {
				return Principal{}, err
			}
		}
		states, err := a.adminSupplierStates()
		if err != nil {
			return Principal{}, err
		}
		for _, item := range customers {
			state := states[strings.TrimSpace(item.ID)]
			codeValue := strings.TrimSpace(state.CustomCode)
			if codeValue == "" {
				continue
			}
			if strings.TrimSpace(code) == codeValue &&
				strings.TrimSpace(item.Phone) != "" &&
				strings.EqualFold(strings.TrimSpace(item.Phone), normalizedPhone) {
				principal := Principal{
					Role:        RoleCustomer,
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
			// Werka account is code-driven. Phone input should not block
			// customer issue / delivery note operations due to formatting drift.
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
	if principal.Role == RoleCustomer {
		doc, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref)
		if err == nil {
			principal.Phone = doc.Phone
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
	case strings.HasPrefix(trimmed, "30"):
		return RoleCustomer, nil
	default:
		return "", ErrInvalidRole
	}
}

func (a *ERPAuthenticator) SupplierHistory(ctx context.Context, principal Principal) ([]DispatchRecord, error) {
	items, err := a.collectSupplierPurchaseReceipts(ctx, principal.Ref)
	if err != nil {
		return nil, err
	}

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
	if a.reader != nil {
		return a.reader.SupplierSummary(ctx, principal.Ref)
	}
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
	customerRecords, err := a.collectWerkaCustomerDeliveryRecords(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range customerRecords {
		if record.Status == "pending" {
			result = append(result, record)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (a *ERPAuthenticator) WerkaHome(ctx context.Context, pendingLimit int) (WerkaHomeData, error) {
	if a.reader != nil {
		return a.reader.WerkaHome(ctx, pendingLimit)
	}
	summary, err := a.WerkaSummary(ctx)
	if err != nil {
		return WerkaHomeData{}, err
	}
	pending, err := a.WerkaPending(ctx, pendingLimit)
	if err != nil {
		return WerkaHomeData{}, err
	}
	return WerkaHomeData{
		Summary:      summary,
		PendingItems: pending,
	}, nil
}

func (a *ERPAuthenticator) WerkaSummary(ctx context.Context) (WerkaHomeSummary, error) {
	if a.reader != nil {
		return a.reader.WerkaSummary(ctx)
	}
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
	customerRecords, err := a.collectWerkaCustomerDeliveryRecords(ctx)
	if err != nil {
		return WerkaHomeSummary{}, err
	}
	for _, record := range customerRecords {
		switch record.Status {
		case "pending":
			summary.PendingCount++
		case "accepted":
			summary.ConfirmedCount++
		case "rejected", "cancelled", "partial":
			summary.ReturnedCount++
		}
	}
	return summary, nil
}

func (a *ERPAuthenticator) WerkaStatusBreakdown(ctx context.Context, kind string) ([]WerkaStatusBreakdownEntry, error) {
	if a.reader != nil {
		return a.reader.WerkaStatusBreakdown(ctx, kind)
	}
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
	customerRecords, err := a.collectWerkaCustomerDeliveryRecords(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range customerRecords {
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
	if a.reader != nil {
		return a.reader.WerkaStatusDetails(ctx, kind, supplierRef)
	}
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
	customerRecords, err := a.collectWerkaCustomerDeliveryRecords(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range customerRecords {
		if needle != "" && !strings.EqualFold(strings.TrimSpace(record.SupplierRef), needle) {
			continue
		}
		if !recordMatchesWerkaBreakdown(record, kind) {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	return result, nil
}

func (a *ERPAuthenticator) WerkaHistory(ctx context.Context) ([]DispatchRecord, error) {
	if a.reader != nil {
		return a.reader.WerkaHistory(ctx)
	}
	return a.collectWerkaHistoryRecords(ctx)
}

func (a *ERPAuthenticator) collectWerkaHistoryRecords(ctx context.Context) ([]DispatchRecord, error) {
	items, err := a.collectTelegramPurchaseReceipts(ctx)
	if err != nil {
		return nil, err
	}

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

	customerResultEvents, err := a.customerResultEvents(ctx)
	if err != nil {
		return nil, err
	}
	result = append(result, customerResultEvents...)

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	return result, nil
}

func (a *ERPAuthenticator) customerResultEvents(ctx context.Context) ([]DispatchRecord, error) {
	customers, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(customers))
	for _, customer := range customers {
		deliveryNotes, err := a.collectCustomerDeliveryNotes(ctx, customer.ID)
		if err != nil {
			return nil, err
		}
		if len(deliveryNotes) == 0 {
			continue
		}
		for _, item := range deliveryNotes {
			record, ok := buildCustomerDeliveryResultEvent(item)
			if !ok {
				continue
			}
			result = append(result, record)
		}
	}
	return result, nil
}

func (a *ERPAuthenticator) collectWerkaCustomerDeliveryRecords(ctx context.Context) ([]DispatchRecord, error) {
	customers, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
	if err != nil {
		return nil, err
	}

	result := make([]DispatchRecord, 0, len(customers))
	for _, customer := range customers {
		deliveryNotes, err := a.collectCustomerDeliveryNotes(ctx, customer.ID)
		if err != nil {
			return nil, err
		}
		for _, item := range deliveryNotes {
			if !customerDeliveryVisible(item) {
				continue
			}
			result = append(result, mapDeliveryNoteToDispatchRecord(item))
		}
	}
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

func (a *ERPAuthenticator) collectCustomerDeliveryNotes(ctx context.Context, customerRef string) ([]erpnext.DeliveryNoteDraft, error) {
	const pageSize = 200
	result := make([]erpnext.DeliveryNoteDraft, 0, pageSize)
	seen := make(map[string]struct{}, pageSize)
	for offset := 0; ; offset += pageSize {
		items, err := a.erp.ListCustomerDeliveryNotesPage(ctx, a.baseURL, a.apiKey, a.apiSecret, customerRef, pageSize, offset)
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
	targetName, targetType, eventType, err := resolveNotificationTarget(receiptID)
	if err != nil {
		return NotificationDetail{}, err
	}
	if targetType == notificationTargetDeliveryNote {
		draft, err := a.erp.GetDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName)
		if err != nil {
			return NotificationDetail{}, err
		}
		if principal.Role == RoleCustomer &&
			strings.TrimSpace(draft.Customer) != strings.TrimSpace(principal.Ref) {
			return NotificationDetail{}, ErrUnauthorized
		}

		comments, err := a.erp.ListDeliveryNoteComments(
			ctx,
			a.baseURL,
			a.apiKey,
			a.apiSecret,
			draft.Name,
			100,
		)
		if err != nil {
			return NotificationDetail{}, err
		}

		record, ok := buildCustomerDeliveryResultEvent(draft)
		if !ok {
			record = mapDeliveryNoteToDispatchRecord(draft)
			record.ID = strings.TrimSpace(receiptID)
		} else if strings.TrimSpace(receiptID) != "" {
			record.ID = strings.TrimSpace(receiptID)
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

	draft, err := a.erp.GetPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName)
	if err != nil {
		return NotificationDetail{}, err
	}
	if principal.Role == RoleCustomer {
		return NotificationDetail{}, ErrUnauthorized
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
	targetName, targetType, _, err := resolveNotificationTarget(receiptID)
	if err != nil {
		return NotificationDetail{}, err
	}

	_, err = a.NotificationDetail(ctx, principal, receiptID)
	if err != nil {
		return NotificationDetail{}, err
	}

	formatted := formatNotificationComment(principal, trimmedMessage)
	switch targetType {
	case notificationTargetDeliveryNote:
		if err := a.erp.AddDeliveryNoteComment(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName, formatted); err != nil {
			return NotificationDetail{}, err
		}
	default:
		if err := a.erp.AddPurchaseReceiptComment(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName, formatted); err != nil {
			return NotificationDetail{}, err
		}
	}
	if principal.Role == RoleSupplier && isSupplierAcknowledgmentMessage(trimmedMessage) {
		draft, err := a.erp.GetPurchaseReceipt(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName)
		if err != nil {
			return NotificationDetail{}, err
		}
		remarks := erpnext.UpsertSupplierAcknowledgmentInRemarks(
			draft.Remarks,
			trimmedMessage,
		)
		if err := a.erp.UpdatePurchaseReceiptRemarks(ctx, a.baseURL, a.apiKey, a.apiSecret, targetName, remarks); err != nil {
			// Supplier acknowledgment is already stored as a comment; remarks backfill is best-effort.
		}
	}
	return a.NotificationDetail(ctx, principal, receiptID)
}

func resolveNotificationTarget(receiptID string) (targetName, targetType, eventType string, err error) {
	trimmedReceiptID := strings.TrimSpace(receiptID)
	if strings.HasPrefix(trimmedReceiptID, supplierAckEventPrefix) {
		eventType = "supplier_ack"
		parts := strings.SplitN(strings.TrimPrefix(trimmedReceiptID, supplierAckEventPrefix), ":", 2)
		if len(parts) > 0 {
			trimmedReceiptID = strings.TrimSpace(parts[0])
		}
	}
	if strings.HasPrefix(trimmedReceiptID, customerDeliveryResultEventPrefix) {
		parts := strings.SplitN(strings.TrimPrefix(trimmedReceiptID, customerDeliveryResultEventPrefix), ":", 2)
		if len(parts) > 0 {
			targetName = strings.TrimSpace(parts[0])
		}
		if targetName == "" {
			return "", "", "", fmt.Errorf("delivery note id is required")
		}
		return targetName, notificationTargetDeliveryNote, eventType, nil
	}
	if trimmedReceiptID == "" {
		return "", "", "", fmt.Errorf("receipt id is required")
	}
	return trimmedReceiptID, notificationTargetPurchaseReceipt, eventType, nil
}

func (a *ERPAuthenticator) WerkaSuppliers(ctx context.Context, query string, limit int) ([]SupplierDirectoryEntry, error) {
	return a.WerkaSuppliersPage(ctx, query, limit, 0)
}

func (a *ERPAuthenticator) WerkaSuppliersPage(ctx context.Context, query string, limit, offset int) ([]SupplierDirectoryEntry, error) {
	if a.reader != nil {
		return a.reader.SearchWerkaSuppliersPage(ctx, query, limit, offset)
	}
	searchLimit := limit + offset
	if searchLimit <= 0 {
		searchLimit = 100
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		searchLimit *= 4
	} else {
		searchLimit = 500
	}
	if searchLimit > 500 {
		searchLimit = 500
	}

	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, query, searchLimit)
	if err != nil {
		return nil, err
	}
	states, err := a.adminSupplierStates()
	if err != nil {
		return nil, err
	}
	result := make([]SupplierDirectoryEntry, 0, len(items))
	for _, item := range items {
		state := states[strings.TrimSpace(item.ID)]
		if state.Removed || state.Blocked {
			continue
		}
		result = append(result, SupplierDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return sliceSupplierDirectoryEntries(result, offset, limit), nil
}

func (a *ERPAuthenticator) WerkaCustomers(ctx context.Context, query string, limit int) ([]CustomerDirectoryEntry, error) {
	return a.WerkaCustomersPage(ctx, query, limit, 0)
}

func (a *ERPAuthenticator) WerkaCustomersPage(ctx context.Context, query string, limit, offset int) ([]CustomerDirectoryEntry, error) {
	if a.reader != nil {
		return a.reader.SearchWerkaCustomersPage(ctx, query, limit, offset)
	}
	searchLimit := limit + offset
	if searchLimit <= 0 {
		searchLimit = 100
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		searchLimit *= 4
	} else {
		searchLimit = 500
	}
	if searchLimit > 500 {
		searchLimit = 500
	}

	items, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, query, searchLimit)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerDirectoryEntry, 0, len(items))
	for _, item := range items {
		customerRef := strings.TrimSpace(item.ID)
		if customerRef == "" {
			continue
		}
		assigned, err := a.erp.ListCustomerItems(ctx, a.baseURL, a.apiKey, a.apiSecret, customerRef, "", 1)
		if err != nil {
			return nil, err
		}
		if len(assigned) == 0 {
			continue
		}
		result = append(result, CustomerDirectoryEntry{
			Ref:   customerRef,
			Name:  item.Name,
			Phone: item.Phone,
		})
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return sliceCustomerDirectoryEntries(result, offset, limit), nil
}

func (a *ERPAuthenticator) WerkaCustomerItems(ctx context.Context, customerRef, query string, limit int) ([]SupplierItem, error) {
	return a.WerkaCustomerItemsPage(ctx, customerRef, query, limit, 0)
}

func (a *ERPAuthenticator) WerkaCustomerItemsPage(ctx context.Context, customerRef, query string, limit, offset int) ([]SupplierItem, error) {
	if a.reader != nil {
		return a.reader.SearchWerkaCustomerItemsPage(ctx, customerRef, query, limit, offset)
	}
	searchLimit := limit + offset
	items, err := a.erp.ListCustomerItems(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(customerRef), query, searchLimit)
	if err != nil {
		return nil, err
	}
	mapped, err := a.mapSupplierItems(ctx, items)
	if err != nil {
		return nil, err
	}
	return sliceSupplierItems(mapped, offset, limit), nil
}

func (a *ERPAuthenticator) WerkaCustomerItemOptions(ctx context.Context, query string, limit int) ([]CustomerItemOption, error) {
	return a.WerkaCustomerItemOptionsPage(ctx, query, limit, 0)
}

func (a *ERPAuthenticator) WerkaCustomerItemOptionsPage(ctx context.Context, query string, limit, offset int) ([]CustomerItemOption, error) {
	if a.reader != nil {
		return a.reader.SearchWerkaCustomerItemOptionsPage(ctx, query, limit, offset)
	}
	customers, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 200)
	if err != nil {
		return nil, err
	}
	customerByRef := make(map[string]erpnext.Customer, len(customers))
	for _, customer := range customers {
		ref := strings.TrimSpace(customer.ID)
		if ref == "" {
			continue
		}
		customerByRef[ref] = customer
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	result := make([]CustomerItemOption, 0, 64)
	seen := make(map[string]struct{})
	searchLimit := limit
	if searchLimit <= 0 {
		searchLimit = 100
	}
	if searchLimit > 200 {
		searchLimit = 200
	}
	candidates, err := a.erp.SearchItems(ctx, a.baseURL, a.apiKey, a.apiSecret, query, searchLimit)
	if err != nil {
		return nil, err
	}
	mapped, err := a.mapSupplierItems(ctx, candidates)
	if err != nil {
		return nil, err
	}

	type itemAssignment struct {
		item         SupplierItem
		customerRefs []string
		err          error
	}

	assignments := make([]itemAssignment, len(mapped))
	workerCount := len(mapped)
	if workerCount > 8 {
		workerCount = 8
	}
	if workerCount > 0 {
		jobs := make(chan int)
		var wg sync.WaitGroup
		for worker := 0; worker < workerCount; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range jobs {
					item := mapped[idx]
					assignment, err := a.erp.GetItemCustomerAssignment(ctx, a.baseURL, a.apiKey, a.apiSecret, item.Code)
					assignments[idx] = itemAssignment{
						item:         item,
						customerRefs: assignment.CustomerRefs,
						err:          err,
					}
				}
			}()
		}
		for idx := range mapped {
			jobs <- idx
		}
		close(jobs)
		wg.Wait()
	}

	for _, assignment := range assignments {
		if assignment.err != nil {
			return nil, assignment.err
		}
		item := assignment.item
		for _, customerRef := range assignment.customerRefs {
			customer, ok := customerByRef[strings.TrimSpace(customerRef)]
			if !ok {
				continue
			}
			if normalizedQuery != "" {
				customerMatches := strings.Contains(strings.ToLower(strings.TrimSpace(customer.Name)), normalizedQuery) ||
					strings.Contains(strings.ToLower(strings.TrimSpace(customer.Phone)), normalizedQuery) ||
					strings.Contains(strings.ToLower(strings.TrimSpace(customer.ID)), normalizedQuery)
				itemMatches := strings.Contains(strings.ToLower(strings.TrimSpace(item.Name)), normalizedQuery) ||
					strings.Contains(strings.ToLower(strings.TrimSpace(item.Code)), normalizedQuery)
				if !customerMatches && !itemMatches {
					continue
				}
			}
			key := strings.TrimSpace(customer.ID) + "|" + strings.TrimSpace(item.Code)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, CustomerItemOption{
				CustomerRef:   strings.TrimSpace(customer.ID),
				CustomerName:  strings.TrimSpace(customer.Name),
				CustomerPhone: strings.TrimSpace(customer.Phone),
				ItemCode:      strings.TrimSpace(item.Code),
				ItemName:      strings.TrimSpace(item.Name),
				UOM:           strings.TrimSpace(item.UOM),
				Warehouse:     strings.TrimSpace(item.Warehouse),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		leftItem := strings.ToLower(result[i].ItemName)
		rightItem := strings.ToLower(result[j].ItemName)
		if leftItem != rightItem {
			return leftItem < rightItem
		}
		leftCustomer := strings.ToLower(result[i].CustomerName)
		rightCustomer := strings.ToLower(result[j].CustomerName)
		if leftCustomer != rightCustomer {
			return leftCustomer < rightCustomer
		}
		return strings.ToLower(result[i].ItemCode) < strings.ToLower(result[j].ItemCode)
	})

	return sliceCustomerItemOptions(result, offset, limit), nil
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
	result, err := a.erp.CreateDraftDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateDeliveryNoteInput{
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
	cleanupDraft := func() {
		if cleanupErr := a.erp.DeleteDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, result.Name); cleanupErr != nil {
			// Best-effort cleanup. The original submit/update error should be returned.
		}
	}
	if err := a.erp.UpdateDeliveryNoteState(
		ctx,
		a.baseURL,
		a.apiKey,
		a.apiSecret,
		result.Name,
		erpnext.DeliveryNoteStateUpdate{
			FlowState:      strconv.Itoa(deliveryFlowStateSubmitted),
			CustomerState:  strconv.Itoa(customerStatePending),
			CustomerReason: "",
			DeliveryActor:  strconv.Itoa(deliveryActorWerka),
			UIStatus:       customerDeliveryUIStatus(deliveryFlowStateSubmitted, customerStatePending),
		},
	); err != nil {
		cleanupDraft()
		return WerkaCustomerIssueRecord{}, err
	}
	if err := a.erp.SubmitDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, result.Name); err != nil {
		cleanupDraft()
		if isERPNegativeStockError(err) {
			return WerkaCustomerIssueRecord{}, ErrInsufficientStock
		}
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
		CreatedLabel: currentTimestampLabel(),
	}, nil
}

func (a *ERPAuthenticator) WerkaSupplierItems(ctx context.Context, supplierRef, query string, limit int) ([]SupplierItem, error) {
	return a.WerkaSupplierItemsPage(ctx, supplierRef, query, limit, 0)
}

func (a *ERPAuthenticator) WerkaSupplierItemsPage(ctx context.Context, supplierRef, query string, limit, offset int) ([]SupplierItem, error) {
	if a.reader != nil {
		return a.reader.SearchWerkaSupplierItemsPage(ctx, supplierRef, query, limit, offset)
	}
	principal := Principal{Role: RoleSupplier, Ref: strings.TrimSpace(supplierRef)}
	items, err := a.supplierAllowedItems(ctx, principal, query, limit+offset)
	if err != nil {
		return nil, err
	}
	return sliceSupplierItems(items, offset, limit), nil
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
	items, err := a.collectWerkaHistoryRecords(ctx)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (a *ERPAuthenticator) CustomerHistory(ctx context.Context, principal Principal) ([]DispatchRecord, error) {
	items, err := a.collectCustomerDeliveryNotes(ctx, principal.Ref)
	if err != nil {
		return nil, err
	}
	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		if !customerDeliveryVisible(item) {
			continue
		}
		result = append(result, mapDeliveryNoteToDispatchRecord(item))
	}
	return result, nil
}

func (a *ERPAuthenticator) CustomerSummary(ctx context.Context, principal Principal) (CustomerHomeSummary, error) {
	if a.reader != nil {
		return a.reader.CustomerSummary(ctx, principal.Ref)
	}
	items, err := a.collectCustomerDeliveryNotes(ctx, principal.Ref)
	if err != nil {
		return CustomerHomeSummary{}, err
	}
	summary := CustomerHomeSummary{}
	for _, item := range items {
		if !customerDeliveryVisible(item) {
			continue
		}
		switch customerDeliveryStatus(item) {
		case "accepted":
			summary.ConfirmedCount++
		case "partial", "rejected":
			summary.RejectedCount++
		default:
			summary.PendingCount++
		}
	}
	return summary, nil
}

func (a *ERPAuthenticator) CustomerStatusDetails(ctx context.Context, principal Principal, kind string) ([]DispatchRecord, error) {
	items, err := a.collectCustomerDeliveryNotes(ctx, principal.Ref)
	if err != nil {
		return nil, err
	}
	filterKind := strings.TrimSpace(kind)
	if filterKind == "confirmed" {
		filterKind = "accepted"
	}
	result := make([]DispatchRecord, 0, len(items))
	for _, item := range items {
		if !customerDeliveryVisible(item) {
			continue
		}
		status := customerDeliveryStatus(item)
		if filterKind == "rejected" {
			if status != "rejected" && status != "partial" {
				continue
			}
		} else if status != filterKind {
			continue
		}
		result = append(result, mapDeliveryNoteToDispatchRecord(item))
	}
	return result, nil
}

func (a *ERPAuthenticator) CustomerDeliveryDetail(ctx context.Context, principal Principal, deliveryNoteID string) (CustomerDeliveryDetail, error) {
	draft, err := a.erp.GetDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, deliveryNoteID)
	if err != nil {
		return CustomerDeliveryDetail{}, err
	}
	if strings.TrimSpace(draft.Customer) != strings.TrimSpace(principal.Ref) {
		return CustomerDeliveryDetail{}, ErrUnauthorized
	}
	status := customerDeliveryStatus(draft)
	pending := status == "pending"
	return CustomerDeliveryDetail{
		Record:             mapDeliveryNoteToDispatchRecord(draft),
		CanApprove:         pending,
		CanReject:          pending,
		CanPartiallyAccept: pending,
		CanReportClaim:     status == "accepted",
	}, nil
}

func (a *ERPAuthenticator) CustomerRespondDelivery(ctx context.Context, principal Principal, deliveryNoteID string, approve bool, reason string) (CustomerDeliveryDetail, error) {
	return a.CustomerRespondDeliveryRequest(ctx, principal, CustomerDeliveryResponseRequest{
		DeliveryNoteID: strings.TrimSpace(deliveryNoteID),
		Approve:        &approve,
		Reason:         strings.TrimSpace(reason),
	})
}

func (a *ERPAuthenticator) CustomerRespondDeliveryRequest(ctx context.Context, principal Principal, req CustomerDeliveryResponseRequest) (CustomerDeliveryDetail, error) {
	deliveryNoteID := strings.TrimSpace(req.DeliveryNoteID)
	draft, err := a.erp.GetDeliveryNote(ctx, a.baseURL, a.apiKey, a.apiSecret, deliveryNoteID)
	if err != nil {
		return CustomerDeliveryDetail{}, err
	}
	if strings.TrimSpace(draft.Customer) != strings.TrimSpace(principal.Ref) {
		return CustomerDeliveryDetail{}, ErrUnauthorized
	}
	decision, err := normalizeCustomerDeliveryDecision(req, draft)
	if err != nil {
		return CustomerDeliveryDetail{}, err
	}

	if decision.returnedQty > 0 {
		if nearlyEqualQty(decision.returnedQty, draft.Qty) {
			if _, err := a.erp.CreateAndSubmitDeliveryNoteReturn(
				ctx,
				a.baseURL,
				a.apiKey,
				a.apiSecret,
				draft.Name,
			); err != nil {
				return CustomerDeliveryDetail{}, err
			}
		} else {
			if _, err := a.erp.CreateAndSubmitPartialDeliveryNoteReturn(
				ctx,
				a.baseURL,
				a.apiKey,
				a.apiSecret,
				draft.Name,
				decision.returnedQty,
			); err != nil {
				return CustomerDeliveryDetail{}, err
			}
		}
	}

	combinedReason := combineCustomerReasonAndComment(decision.reason, decision.comment)
	remarks := erpnext.UpsertCustomerDecisionPayloadInRemarks(
		draft.Remarks,
		decision.stateLabel(),
		decision.reason,
		decision.acceptedQty,
		decision.returnedQty,
		draft.UOM,
		decision.comment,
	)
	if remarks != strings.TrimSpace(draft.Remarks) {
		if err := a.erp.UpdateDeliveryNoteRemarks(
			ctx,
			a.baseURL,
			a.apiKey,
			a.apiSecret,
			draft.Name,
			remarks,
		); err != nil {
			return CustomerDeliveryDetail{}, err
		}
	}
	if err := a.erp.UpdateDeliveryNoteState(
		ctx,
		a.baseURL,
		a.apiKey,
		a.apiSecret,
		draft.Name,
		erpnext.DeliveryNoteStateUpdate{
			FlowState:      strconv.Itoa(deliveryFlowStateSubmitted),
			CustomerState:  strconv.Itoa(decision.customerState),
			CustomerReason: combinedReason,
			DeliveryActor:  strconv.Itoa(deliveryActorWerka),
			UIStatus:       customerDeliveryUIStatus(deliveryFlowStateSubmitted, decision.customerState),
		},
	); err != nil {
		return CustomerDeliveryDetail{}, err
	}
	draft.Remarks = remarks
	draft.AccordFlowState = strconv.Itoa(deliveryFlowStateSubmitted)
	draft.AccordCustomerState = strconv.Itoa(decision.customerState)
	draft.AccordCustomerReason = combinedReason
	draft.AccordDeliveryActor = strconv.Itoa(deliveryActorWerka)
	draft.AccordUIStatus = customerDeliveryUIStatus(
		deliveryFlowStateSubmitted,
		decision.customerState,
	)
	return CustomerDeliveryDetail{
		Record:             mapDeliveryNoteToDispatchRecord(draft),
		CanApprove:         false,
		CanReject:          false,
		CanPartiallyAccept: false,
		CanReportClaim:     false,
	}, nil
}

func customerDeliveryStatus(item erpnext.DeliveryNoteDraft) string {
	if item.DocStatus != 1 {
		return "draft"
	}
	if deliveryFlowStateValue(item) != deliveryFlowStateSubmitted {
		return "pending"
	}
	switch customerStateValue(item) {
	case customerStateRejected:
		return "rejected"
	case customerStateConfirmed:
		return "accepted"
	case customerStatePartial:
		return "partial"
	default:
		return "pending"
	}
}

func customerDeliveryVisible(item erpnext.DeliveryNoteDraft) bool {
	return item.DocStatus == 1 && deliveryFlowStateValue(item) == deliveryFlowStateSubmitted
}

func customerDeliveryUIStatus(flowState, customerState int) string {
	if flowState != deliveryFlowStateSubmitted {
		return "pending"
	}
	switch customerState {
	case customerStateConfirmed:
		return "confirm"
	case customerStatePartial:
		return "partial"
	case customerStateRejected:
		return "rejected"
	default:
		return "pending"
	}
}

func buildCustomerDeliveryResultEvent(item erpnext.DeliveryNoteDraft) (DispatchRecord, bool) {
	state := customerDeliveryStatus(item)
	if state != "accepted" && state != "partial" && state != "rejected" {
		return DispatchRecord{}, false
	}

	base := mapDeliveryNoteToDispatchRecord(item)
	base.ID = customerDeliveryResultEventPrefix + strings.TrimSpace(item.Name)
	if state == "accepted" {
		base.EventType = "customer_delivery_confirmed"
		base.Highlight = "Customer mahsulotni qabul qildi"
		return base, true
	}
	if state == "partial" {
		base.EventType = "customer_delivery_partial"
		base.Highlight = "Customer mahsulotning bir qismini qaytardi"
		return base, true
	}

	base.EventType = "customer_delivery_rejected"
	base.Highlight = "Customer mahsulotni rad etdi"
	return base, true
}

type customerDeliveryDecision struct {
	mode          CustomerDeliveryResponseMode
	customerState int
	acceptedQty   float64
	returnedQty   float64
	reason        string
	comment       string
}

func (d customerDeliveryDecision) stateLabel() string {
	switch d.customerState {
	case customerStateConfirmed:
		return "confirmed"
	case customerStateRejected:
		return "rejected"
	case customerStatePartial:
		return "partial"
	default:
		return "pending"
	}
}

func normalizeCustomerDeliveryDecision(req CustomerDeliveryResponseRequest, draft erpnext.DeliveryNoteDraft) (customerDeliveryDecision, error) {
	currentStatus := customerDeliveryStatus(draft)
	mode := strings.TrimSpace(string(req.Mode))
	if mode == "" && req.Approve != nil {
		if *req.Approve {
			mode = string(CustomerDeliveryResponseAcceptAll)
		} else {
			mode = string(CustomerDeliveryResponseRejectAll)
		}
	}

	reason := strings.TrimSpace(req.Reason)
	comment := strings.TrimSpace(req.Comment)
	sentQty := draft.Qty
	if sentQty <= 0 {
		return customerDeliveryDecision{}, ErrInvalidInput
	}

	switch CustomerDeliveryResponseMode(mode) {
	case CustomerDeliveryResponseAcceptAll:
		if currentStatus != "pending" {
			return customerDeliveryDecision{}, fmt.Errorf("delivery note is not pending")
		}
		return customerDeliveryDecision{
			mode:          CustomerDeliveryResponseAcceptAll,
			customerState: customerStateConfirmed,
			acceptedQty:   sentQty,
			reason:        reason,
			comment:       comment,
		}, nil
	case CustomerDeliveryResponseRejectAll:
		if currentStatus != "pending" {
			return customerDeliveryDecision{}, fmt.Errorf("delivery note is not pending")
		}
		if !hasMeaningfulCustomerReturnReason(reason, comment) {
			return customerDeliveryDecision{}, ErrInvalidInput
		}
		return customerDeliveryDecision{
			mode:          CustomerDeliveryResponseRejectAll,
			customerState: customerStateRejected,
			returnedQty:   sentQty,
			reason:        reason,
			comment:       comment,
		}, nil
	case CustomerDeliveryResponseAcceptPartial:
		if currentStatus != "pending" {
			return customerDeliveryDecision{}, fmt.Errorf("delivery note is not pending")
		}
		if !hasMeaningfulCustomerReturnReason(reason, comment) {
			return customerDeliveryDecision{}, ErrInvalidInput
		}
		acceptedQty, returnedQty, err := normalizePartialQuantities(sentQty, req.AcceptedQty, req.ReturnedQty)
		if err != nil {
			return customerDeliveryDecision{}, err
		}
		return customerDeliveryDecision{
			mode:          CustomerDeliveryResponseAcceptPartial,
			customerState: customerStatePartial,
			acceptedQty:   acceptedQty,
			returnedQty:   returnedQty,
			reason:        reason,
			comment:       comment,
		}, nil
	case CustomerDeliveryResponseClaimAfterAccept:
		if currentStatus != "accepted" {
			return customerDeliveryDecision{}, fmt.Errorf("delivery note cannot accept claim in status %s", currentStatus)
		}
		if !hasMeaningfulCustomerReturnReason(reason, comment) {
			return customerDeliveryDecision{}, ErrInvalidInput
		}
		returnedQty := req.ReturnedQty
		if returnedQty <= 0 || returnedQty > sentQty+customerQtyTolerance {
			return customerDeliveryDecision{}, ErrInvalidInput
		}
		if nearlyEqualQty(returnedQty, sentQty) {
			return customerDeliveryDecision{
				mode:          CustomerDeliveryResponseClaimAfterAccept,
				customerState: customerStateRejected,
				returnedQty:   sentQty,
				reason:        reason,
				comment:       comment,
			}, nil
		}
		return customerDeliveryDecision{
			mode:          CustomerDeliveryResponseClaimAfterAccept,
			customerState: customerStatePartial,
			acceptedQty:   sentQty - returnedQty,
			returnedQty:   returnedQty,
			reason:        reason,
			comment:       comment,
		}, nil
	default:
		return customerDeliveryDecision{}, ErrInvalidInput
	}
}

func normalizePartialQuantities(sentQty, acceptedQty, returnedQty float64) (float64, float64, error) {
	switch {
	case acceptedQty > 0 && returnedQty > 0:
	case acceptedQty > 0:
		returnedQty = sentQty - acceptedQty
	case returnedQty > 0:
		acceptedQty = sentQty - returnedQty
	default:
		return 0, 0, ErrInvalidInput
	}
	if acceptedQty <= 0 || returnedQty <= 0 {
		return 0, 0, ErrInvalidInput
	}
	if math.Abs((acceptedQty+returnedQty)-sentQty) > customerQtyTolerance {
		return 0, 0, ErrInvalidInput
	}
	return acceptedQty, returnedQty, nil
}

func nearlyEqualQty(left, right float64) bool {
	return math.Abs(left-right) <= customerQtyTolerance
}

func hasMeaningfulCustomerReturnReason(reason, comment string) bool {
	if utf8.RuneCountInString(strings.TrimSpace(reason)) >= minCustomerRejectReasonRunes {
		return true
	}
	return utf8.RuneCountInString(strings.TrimSpace(comment)) >= minCustomerRejectReasonRunes
}

func combineCustomerReasonAndComment(reason, comment string) string {
	trimmedReason := strings.TrimSpace(reason)
	trimmedComment := strings.TrimSpace(comment)
	switch {
	case trimmedReason == "":
		return trimmedComment
	case trimmedComment == "":
		return trimmedReason
	default:
		return trimmedReason + ". " + trimmedComment
	}
}

func isERPNegativeStockError(err error) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "negativestockerror")
}

func deliveryFlowStateValue(item erpnext.DeliveryNoteDraft) int {
	return parseAccordInt(item.AccordFlowState, deliveryFlowStateNone)
}

func customerStateValue(item erpnext.DeliveryNoteDraft) int {
	return parseAccordInt(item.AccordCustomerState, customerStatePending)
}

func parseAccordInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func deliveryNoteNames(items []erpnext.DeliveryNoteDraft) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item.Name); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
		CreatedLabel: currentTimestampLabel(),
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
		CreatedLabel: currentTimestampLabel(),
	}, nil
}

func (a *ERPAuthenticator) AdminSettings() AdminSettings {
	werkaState, _ := a.adminSupplierState(werkaStateRef)
	defaultUOM := strings.TrimSpace(a.defaultUOM)
	if defaultUOM == "" {
		defaultUOM = "Kg"
	}
	return AdminSettings{
		ERPURL:                 a.baseURL,
		ERPAPIKey:              a.apiKey,
		ERPAPISecret:           a.apiSecret,
		DefaultTargetWarehouse: a.defaultWarehouse,
		DefaultUOM:             defaultUOM,
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
	a.defaultUOM = strings.TrimSpace(input.DefaultUOM)
	if a.defaultUOM == "" {
		a.defaultUOM = "Kg"
	}
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
			"ERP_DEFAULT_UOM":              a.defaultUOM,
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

func currentTimestampLabel() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func normalizeConfigPhone(phone string) (string, error) {
	cleanPhone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(phone)
	if !strings.HasPrefix(strings.TrimSpace(cleanPhone), "+") {
		digitsOnly := cleanPhone
		if len(digitsOnly) == 9 {
			cleanPhone = "998" + digitsOnly
		}
	}
	return suplier.NormalizePhone(cleanPhone)
}

func phonesLooselyMatch(left, right string) bool {
	leftDigits := onlyDigits(left)
	rightDigits := onlyDigits(right)
	if leftDigits == "" || rightDigits == "" {
		return false
	}
	if leftDigits == rightDigits {
		return true
	}
	if len(leftDigits) >= 9 && len(rightDigits) >= 9 {
		return leftDigits[len(leftDigits)-9:] == rightDigits[len(rightDigits)-9:]
	}
	return false
}

func onlyDigits(value string) string {
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
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
	mu       sync.Mutex
	sessions map[string]sessionRecord
	path     string
	ttl      time.Duration
	loaded   bool
}

type sessionRecord struct {
	Principal Principal `json:"principal"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]sessionRecord),
	}
}

func NewPersistentSessionManager(path string, ttl time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]sessionRecord),
		path:     strings.TrimSpace(path),
		ttl:      ttl,
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

	all, err := m.loadAllLocked()
	if err != nil {
		return "", err
	}
	all[token] = m.newSessionRecord(principal, time.Now().UTC(), time.Time{})
	if err := m.writeAllLocked(all); err != nil {
		return "", err
	}
	return token, nil
}

func (m *SessionManager) Get(token string) (Principal, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	all, err := m.loadAllLocked()
	if err != nil {
		return Principal{}, false
	}
	record, ok := all[token]
	if !ok {
		return Principal{}, false
	}
	if m.isExpired(record, time.Now().UTC()) {
		delete(all, token)
		_ = m.writeAllLocked(all)
		return Principal{}, false
	}
	return record.Principal, true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	all, err := m.loadAllLocked()
	if err != nil {
		return
	}
	if _, ok := all[token]; !ok {
		return
	}
	delete(all, token)
	_ = m.writeAllLocked(all)
}

func (m *SessionManager) Update(token string, principal Principal) {
	m.mu.Lock()
	defer m.mu.Unlock()

	all, err := m.loadAllLocked()
	if err != nil {
		return
	}
	record, ok := all[token]
	if !ok {
		return
	}
	all[token] = m.newSessionRecord(principal, time.Now().UTC(), record.CreatedAt)
	_ = m.writeAllLocked(all)
}

func (m *SessionManager) newSessionRecord(principal Principal, now, createdAt time.Time) sessionRecord {
	if createdAt.IsZero() {
		createdAt = now
	}
	record := sessionRecord{
		Principal: principal,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}
	if m.ttl > 0 {
		record.ExpiresAt = now.Add(m.ttl)
	}
	return record
}

func (m *SessionManager) isExpired(record sessionRecord, now time.Time) bool {
	return !record.ExpiresAt.IsZero() && now.After(record.ExpiresAt)
}

func (m *SessionManager) loadAllLocked() (map[string]sessionRecord, error) {
	if m.loaded {
		if m.sessions == nil {
			m.sessions = map[string]sessionRecord{}
		}
		return m.sessions, nil
	}

	all, err := m.readAllLocked()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for token, record := range all {
		if m.isExpired(record, now) {
			delete(all, token)
		}
	}
	m.sessions = all
	m.loaded = true
	return m.sessions, nil
}

func (m *SessionManager) readAllLocked() (map[string]sessionRecord, error) {
	if m.path == "" {
		return map[string]sessionRecord{}, nil
	}
	if _, err := os.Stat(m.path); err != nil {
		if os.IsNotExist(err) {
			return map[string]sessionRecord{}, nil
		}
		return nil, err
	}

	raw, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]sessionRecord{}, nil
	}

	var data map[string]sessionRecord
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]sessionRecord{}
	}
	return data, nil
}

func (m *SessionManager) writeAllLocked(data map[string]sessionRecord) error {
	m.sessions = cloneSessionRecordMap(data)
	m.loaded = true
	if m.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(m.path), "sessions-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, m.path); err != nil {
		return err
	}
	return nil
}

func cloneSessionRecordMap(input map[string]sessionRecord) map[string]sessionRecord {
	cloned := make(map[string]sessionRecord, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
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
	unannouncedState := strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(item.Remarks))
	if item.DocStatus == 0 && unannouncedState == "pending" {
		eventType = "werka_unannounced_pending"
	} else if status == "accepted" && unannouncedState == "approved" {
		eventType = "werka_unannounced_approved"
		if note == "" {
			note = "Aytilmagan mol tasdiqlandi."
		}
	}
	return DispatchRecord{
		ID:           item.Name,
		RecordType:   "purchase_receipt",
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

func mapDeliveryNoteToDispatchRecord(item erpnext.DeliveryNoteDraft) DispatchRecord {
	status := customerDeliveryStatus(item)
	acceptedQty, returnedQty := customerDecisionQuantities(item, status)
	note := ""
	switch status {
	case "accepted":
		note = "Customer tasdiqladi."
	case "partial":
		note = fmt.Sprintf(
			"Customer qisman qabul qildi. Qabul: %.2f %s. Qaytdi: %.2f %s.",
			acceptedQty,
			item.UOM,
			returnedQty,
			item.UOM,
		)
	case "rejected":
		note = "Customer rad etdi."
	}
	if reason := strings.TrimSpace(item.AccordCustomerReason); reason != "" {
		note += " Sabab: " + reason
	}
	return DispatchRecord{
		ID:           item.Name,
		RecordType:   "delivery_note",
		SupplierRef:  item.Customer,
		SupplierName: item.CustomerName,
		ItemCode:     item.ItemCode,
		ItemName:     item.ItemName,
		UOM:          item.UOM,
		SentQty:      item.Qty,
		AcceptedQty:  acceptedQty,
		Note:         note,
		Status:       status,
		CreatedLabel: firstNonEmpty(item.Modified, item.PostingDate),
	}
}

func customerDecisionQuantities(item erpnext.DeliveryNoteDraft, status string) (acceptedQty, returnedQty float64) {
	acceptedQty, returnedQty = erpnext.ExtractCustomerDecisionQuantities(item.Remarks)
	if returnedQty <= 0 && item.ReturnedQty > 0 {
		returnedQty = item.ReturnedQty
	}
	switch status {
	case "accepted":
		if acceptedQty <= 0 {
			acceptedQty = item.Qty
		}
		return acceptedQty, 0
	case "partial":
		if acceptedQty <= 0 && returnedQty > 0 {
			acceptedQty = maxFloat(item.Qty-returnedQty, 0)
		}
		if returnedQty <= 0 && acceptedQty > 0 {
			returnedQty = maxFloat(item.Qty-acceptedQty, 0)
		}
		return acceptedQty, returnedQty
	case "rejected":
		return 0, item.Qty
	default:
		return acceptedQty, returnedQty
	}
}

func formatNotificationComment(principal Principal, message string) string {
	label := "Tizim"
	switch principal.Role {
	case RoleSupplier:
		label = "Supplier"
	case RoleWerka:
		label = "Werka"
	case RoleCustomer:
		label = "Customer"
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
		if body != "" &&
			(strings.HasPrefix(head, "Supplier") ||
				strings.HasPrefix(head, "Werka") ||
				strings.HasPrefix(head, "Customer") ||
				strings.HasPrefix(head, "Admin")) {
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

func sliceSupplierDirectoryEntries(items []SupplierDirectoryEntry, offset, limit int) []SupplierDirectoryEntry {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []SupplierDirectoryEntry{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func sliceCustomerDirectoryEntries(items []CustomerDirectoryEntry, offset, limit int) []CustomerDirectoryEntry {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []CustomerDirectoryEntry{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func sliceSupplierItems(items []SupplierItem, offset, limit int) []SupplierItem {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []SupplierItem{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func sliceCustomerItemOptions(items []CustomerItemOption, offset, limit int) []CustomerItemOption {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []CustomerItemOption{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

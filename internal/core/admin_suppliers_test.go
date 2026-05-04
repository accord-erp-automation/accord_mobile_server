package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mobile_server/internal/erpnext"
)

type adminSuppliersERPStub struct {
	searchItems                 func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error)
	searchCustomers             func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error)
	searchSuppliers             func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	searchItemGroups            func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.ItemGroup, error)
	getSupplier                 func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error)
	getCustomer                 func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Customer, error)
	getItemCustomerAssignment   func(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (erpnext.ItemCustomerAssignment, error)
	listCustomerItems           func(ctx context.Context, baseURL, apiKey, apiSecret, customerRef, query string, limit int) ([]erpnext.Item, error)
	getDeliveryNote             func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error)
	listDeliveryNoteComments    func(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error)
	addDeliveryNoteComment      func(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error
	createDraftDeliveryNote     func(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error)
	updateDeliveryNoteState     func(ctx context.Context, baseURL, apiKey, apiSecret, name string, update erpnext.DeliveryNoteStateUpdate) error
	submitDeliveryNote          func(ctx context.Context, baseURL, apiKey, apiSecret, name string) error
	deleteDeliveryNote          func(ctx context.Context, baseURL, apiKey, apiSecret, name string) error
	createDeliveryNoteReturn    func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (erpnext.DeliveryNoteResult, error)
	createPartialDeliveryReturn func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, returnedQty float64) (erpnext.DeliveryNoteResult, error)
	listAssignedSupplierItems   func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error)
	getItemsByCodes             func(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error)
	searchCompanies             func(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error)
	searchWarehouses            func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	updateSupplierContact       func(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error
	addPurchaseReceiptComment   func(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error
}

func (s *adminSuppliersERPStub) SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error) {
	if s.searchItems != nil {
		return s.searchItems(ctx, baseURL, apiKey, apiSecret, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error) {
	if s.searchCustomers != nil {
		return s.searchCustomers(ctx, baseURL, apiKey, apiSecret, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchCompanies(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error) {
	if s.searchCompanies != nil {
		return s.searchCompanies(ctx, baseURL, apiKey, apiSecret, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchItemGroups(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.ItemGroup, error) {
	if s.searchItemGroups != nil {
		return s.searchItemGroups(ctx, baseURL, apiKey, apiSecret, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) GetCustomer(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Customer, error) {
	if s.getCustomer != nil {
		return s.getCustomer(ctx, baseURL, apiKey, apiSecret, id)
	}
	return erpnext.Customer{}, nil
}

func (s *adminSuppliersERPStub) EnsureCustomer(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateCustomerInput) (erpnext.Customer, error) {
	return erpnext.Customer{
		ID:    input.Name,
		Name:  input.Name,
		Phone: input.Phone,
	}, nil
}

func (s *adminSuppliersERPStub) UpdateCustomerDetails(ctx context.Context, baseURL, apiKey, apiSecret, id, details string) error {
	return nil
}

func (s *adminSuppliersERPStub) UpdateCustomerContact(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error {
	return nil
}

func (s *adminSuppliersERPStub) SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error) {
	if s.searchSuppliers != nil {
		return s.searchSuppliers(ctx, baseURL, apiKey, apiSecret, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) GetSupplier(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error) {
	if s.getSupplier != nil {
		return s.getSupplier(ctx, baseURL, apiKey, apiSecret, id)
	}
	return erpnext.Supplier{}, nil
}

func (s *adminSuppliersERPStub) UpdateSupplierDetails(ctx context.Context, baseURL, apiKey, apiSecret, id, details string) error {
	return nil
}

func (s *adminSuppliersERPStub) UpdateSupplierContact(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error {
	if s.updateSupplierContact != nil {
		return s.updateSupplierContact(ctx, baseURL, apiKey, apiSecret, id, phone, details)
	}
	return nil
}

func (s *adminSuppliersERPStub) GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error) {
	if s.getItemsByCodes != nil {
		return s.getItemsByCodes(ctx, baseURL, apiKey, apiSecret, itemCodes)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) CreateItem(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateItemInput) (erpnext.Item, error) {
	return erpnext.Item{}, nil
}

func (s *adminSuppliersERPStub) EnsureSupplier(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateSupplierInput) (erpnext.Supplier, error) {
	return erpnext.Supplier{}, nil
}

func (s *adminSuppliersERPStub) SearchWarehouses(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error) {
	if s.searchWarehouses != nil {
		return s.searchWarehouses(ctx, baseURL, apiKey, apiSecret, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]erpnext.Item, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListCustomerItems(ctx context.Context, baseURL, apiKey, apiSecret, customerRef, query string, limit int) ([]erpnext.Item, error) {
	if s.listCustomerItems != nil {
		return s.listCustomerItems(ctx, baseURL, apiKey, apiSecret, customerRef, query, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) GetItemCustomerAssignment(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (erpnext.ItemCustomerAssignment, error) {
	if s.getItemCustomerAssignment != nil {
		return s.getItemCustomerAssignment(ctx, baseURL, apiKey, apiSecret, itemCode)
	}
	return erpnext.ItemCustomerAssignment{}, nil
}

func (s *adminSuppliersERPStub) ListCustomerDeliveryNotes(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit int) ([]erpnext.DeliveryNoteDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListCustomerDeliveryNotesPage(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit, offset int) ([]erpnext.DeliveryNoteDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) GetDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
	if s.getDeliveryNote != nil {
		return s.getDeliveryNote(ctx, baseURL, apiKey, apiSecret, name)
	}
	return erpnext.DeliveryNoteDraft{}, nil
}

func (s *adminSuppliersERPStub) ListDeliveryNoteComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error) {
	if s.listDeliveryNoteComments != nil {
		return s.listDeliveryNoteComments(ctx, baseURL, apiKey, apiSecret, name, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) ListDeliveryNoteCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]erpnext.Comment, error) {
	return map[string][]erpnext.Comment{}, nil
}

func (s *adminSuppliersERPStub) EnsureDeliveryNoteStateFields(ctx context.Context, baseURL, apiKey, apiSecret string) error {
	return nil
}

func (s *adminSuppliersERPStub) UpdateDeliveryNoteState(ctx context.Context, baseURL, apiKey, apiSecret, name string, update erpnext.DeliveryNoteStateUpdate) error {
	if s.updateDeliveryNoteState != nil {
		return s.updateDeliveryNoteState(ctx, baseURL, apiKey, apiSecret, name, update)
	}
	return nil
}

func (s *adminSuppliersERPStub) ListAssignedSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error) {
	if s.listAssignedSupplierItems != nil {
		return s.listAssignedSupplierItems(ctx, baseURL, apiKey, apiSecret, supplier, limit)
	}
	return nil, nil
}

func (s *adminSuppliersERPStub) AssignSupplierToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	return nil
}

func (s *adminSuppliersERPStub) RemoveSupplierFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	return nil
}

func (s *adminSuppliersERPStub) AssignCustomerToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error {
	return nil
}

func (s *adminSuppliersERPStub) RemoveCustomerFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error {
	return nil
}

func (s *adminSuppliersERPStub) ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListPendingPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListTelegramPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListTelegramPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListSupplierPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) GetPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{}, nil
}

func (s *adminSuppliersERPStub) ListPurchaseReceiptComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) ListPurchaseReceiptCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]erpnext.Comment, error) {
	return map[string][]erpnext.Comment{}, nil
}

func (s *adminSuppliersERPStub) AddPurchaseReceiptComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
	if s.addPurchaseReceiptComment != nil {
		return s.addPurchaseReceiptComment(ctx, baseURL, apiKey, apiSecret, name, content)
	}
	return nil
}

func (s *adminSuppliersERPStub) UpdatePurchaseReceiptRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error {
	return nil
}

func (s *adminSuppliersERPStub) DownloadFile(ctx context.Context, baseURL, apiKey, apiSecret, fileURL string) (string, []byte, error) {
	return "", nil, nil
}

func (s *adminSuppliersERPStub) CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{}, nil
}

func (s *adminSuppliersERPStub) CreateAndSubmitStockEntry(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error) {
	return erpnext.StockEntryResult{}, nil
}

func (s *adminSuppliersERPStub) CreateAndSubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
	return erpnext.DeliveryNoteResult{}, nil
}

func (s *adminSuppliersERPStub) CreateDraftDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
	if s.createDraftDeliveryNote != nil {
		return s.createDraftDeliveryNote(ctx, baseURL, apiKey, apiSecret, input)
	}
	return erpnext.DeliveryNoteResult{}, nil
}

func (s *adminSuppliersERPStub) CreateAndSubmitDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (erpnext.DeliveryNoteResult, error) {
	if s.createDeliveryNoteReturn != nil {
		return s.createDeliveryNoteReturn(ctx, baseURL, apiKey, apiSecret, sourceName)
	}
	return erpnext.DeliveryNoteResult{Name: "RET-DN-0001"}, nil
}

func (s *adminSuppliersERPStub) CreateAndSubmitPartialDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, returnedQty float64) (erpnext.DeliveryNoteResult, error) {
	if s.createPartialDeliveryReturn != nil {
		return s.createPartialDeliveryReturn(ctx, baseURL, apiKey, apiSecret, sourceName, returnedQty)
	}
	return erpnext.DeliveryNoteResult{Name: "RET-DN-0001"}, nil
}

func (s *adminSuppliersERPStub) SubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
	if s.submitDeliveryNote != nil {
		return s.submitDeliveryNote(ctx, baseURL, apiKey, apiSecret, name)
	}
	return nil
}

func (s *adminSuppliersERPStub) UpdateDeliveryNoteRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error {
	return nil
}

func (s *adminSuppliersERPStub) AddDeliveryNoteComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
	if s.addDeliveryNoteComment != nil {
		return s.addDeliveryNoteComment(ctx, baseURL, apiKey, apiSecret, name, content)
	}
	return nil
}

func (s *adminSuppliersERPStub) DeleteDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
	if s.deleteDeliveryNote != nil {
		return s.deleteDeliveryNote(ctx, baseURL, apiKey, apiSecret, name)
	}
	return nil
}

func (s *adminSuppliersERPStub) ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason, returnComment string) (erpnext.PurchaseReceiptSubmissionResult, error) {
	return erpnext.PurchaseReceiptSubmissionResult{}, nil
}

func (s *adminSuppliersERPStub) UploadSupplierImage(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error) {
	return "", nil
}

func TestAdminSupplierDetailFallsBackWhenItemSupplierPermissionDenied(t *testing.T) {
	tempDir := t.TempDir()
	store := NewAdminSupplierStore(filepath.Join(tempDir, "admin-suppliers.json"))
	if err := store.Put("SUP-001", AdminSupplierState{
		AssignmentsConfigured: true,
		AssignedItemCodes:     []string{"ITEM-001"},
	}); err != nil {
		t.Fatalf("seed admin supplier state: %v", err)
	}

	stub := &adminSuppliersERPStub{
		getSupplier: func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error) {
			return erpnext.Supplier{ID: id, Name: "Supplier", Phone: "+998900000001"}, nil
		},
		listAssignedSupplierItems: func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error) {
			return nil, errors.New(`status 403: {"exception":"frappe.exceptions.PermissionError","exc":"check_parent_permission","route":"Item%20Supplier"}`)
		},
		getItemsByCodes: func(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error) {
			return []erpnext.Item{{Code: "ITEM-001", Name: "Bolt", UOM: "Nos"}}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		store,
	)

	detail, err := auth.AdminSupplierDetail(context.Background(), "SUP-001")
	if err != nil {
		t.Fatalf("AdminSupplierDetail() error = %v", err)
	}
	if len(detail.AssignedItems) != 1 {
		t.Fatalf("expected 1 assigned item, got %d", len(detail.AssignedItems))
	}
	if detail.AssignedItems[0].Code != "ITEM-001" {
		t.Fatalf("expected ITEM-001, got %q", detail.AssignedItems[0].Code)
	}
}

func TestNotificationDetailSupportsCustomerDeliveryResultEvents(t *testing.T) {
	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 3,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "3",
			}, nil
		},
		listDeliveryNoteComments: func(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error) {
			return []erpnext.Comment{
				{
					ID:        "LIFECYCLE-1",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-15 09:59:00",
				},
				{
					ID:        "COMMENT-1",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "confirmed", ""),
					CreatedAt: "2026-03-15 10:00:00",
				},
			}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	detail, err := auth.NotificationDetail(
		context.Background(),
		Principal{Role: RoleWerka, Ref: "werka"},
		customerDeliveryResultEventPrefix+"MAT-DN-0001:COMMENT-1",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Record.EventType != "customer_delivery_confirmed" {
		t.Fatalf("unexpected event type: %q", detail.Record.EventType)
	}
	if detail.Record.ID != customerDeliveryResultEventPrefix+"MAT-DN-0001:COMMENT-1" {
		t.Fatalf("unexpected record id: %q", detail.Record.ID)
	}
}

func TestAddNotificationCommentUsesDeliveryNotePathForCustomerDeliveryResultEvents(t *testing.T) {
	var (
		addedDeliveryNoteName string
		addedPurchaseNoteName string
	)
	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 3,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "3",
			}, nil
		},
		listDeliveryNoteComments: func(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error) {
			return []erpnext.Comment{}, nil
		},
		addDeliveryNoteComment: func(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
			addedDeliveryNoteName = name
			return nil
		},
		addPurchaseReceiptComment: func(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
			addedPurchaseNoteName = name
			return nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.AddNotificationComment(
		context.Background(),
		Principal{Role: RoleWerka, Ref: "werka"},
		customerDeliveryResultEventPrefix+"MAT-DN-0001:COMMENT-1",
		"Qabul bo'ldi deb tasdiqladim",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addedDeliveryNoteName != "MAT-DN-0001" {
		t.Fatalf("expected delivery note comment target MAT-DN-0001, got %q", addedDeliveryNoteName)
	}
	if addedPurchaseNoteName != "" {
		t.Fatalf("expected purchase receipt comment path to stay unused, got %q", addedPurchaseNoteName)
	}
}

func TestWerkaLoginDoesNotBlockOnPhoneFormat(t *testing.T) {
	auth := NewERPAuthenticator(
		&adminSuppliersERPStub{},
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"20ABCDEF1234",
		"888862440",
		"Werka",
		nil,
		nil,
	)

	principal, err := auth.Login(context.Background(), "+99888862440", "20ABCDEF1234")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if principal.Role != RoleWerka {
		t.Fatalf("expected werka role, got %q", principal.Role)
	}

	principal, err = auth.Login(context.Background(), "+123456789", "20ABCDEF1234")
	if err != nil {
		t.Fatalf("Login() with alternative format error = %v", err)
	}
	if principal.Role != RoleWerka {
		t.Fatalf("expected werka role for alternative format, got %q", principal.Role)
	}
}

func TestCustomerRespondDeliveryRejectRequiresReason(t *testing.T) {
	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 3,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "1",
			}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.CustomerRespondDelivery(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		"MAT-DN-0001",
		false,
		"ok",
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCustomerRespondDeliveryRejectCreatesReturnDeliveryNote(t *testing.T) {
	var returnAgainst string

	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 3,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "1",
			}, nil
		},
		createDeliveryNoteReturn: func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (erpnext.DeliveryNoteResult, error) {
			returnAgainst = sourceName
			return erpnext.DeliveryNoteResult{Name: "RET-DN-0001"}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.CustomerRespondDelivery(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		"MAT-DN-0001",
		false,
		"Qabul qilinmadi",
	)
	if err != nil {
		t.Fatalf("CustomerRespondDelivery() error = %v", err)
	}
	if returnAgainst != "MAT-DN-0001" {
		t.Fatalf("expected return against MAT-DN-0001, got %q", returnAgainst)
	}
}

func TestCustomerRespondDeliveryApproveDoesNotCreateReturnDeliveryNote(t *testing.T) {
	returnCalled := false

	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 3,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "1",
			}, nil
		},
		createDeliveryNoteReturn: func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (erpnext.DeliveryNoteResult, error) {
			returnCalled = true
			return erpnext.DeliveryNoteResult{Name: "RET-DN-0001"}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.CustomerRespondDelivery(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		"MAT-DN-0001",
		true,
		"",
	)
	if err != nil {
		t.Fatalf("CustomerRespondDelivery() error = %v", err)
	}
	if returnCalled {
		t.Fatal("return delivery note should not be created on approve")
	}
}

func TestCustomerRespondDeliveryPartialCreatesQtyAwareReturnDeliveryNote(t *testing.T) {
	var returnAgainst string
	var returnedQty float64

	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 10,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "1",
			}, nil
		},
		createPartialDeliveryReturn: func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, qty float64) (erpnext.DeliveryNoteResult, error) {
			returnAgainst = sourceName
			returnedQty = qty
			return erpnext.DeliveryNoteResult{Name: "RET-DN-0001"}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	detail, err := auth.CustomerRespondDeliveryRequest(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		CustomerDeliveryResponseRequest{
			DeliveryNoteID: "MAT-DN-0001",
			Mode:           CustomerDeliveryResponseAcceptPartial,
			AcceptedQty:    7,
			ReturnedQty:    3,
			Reason:         "Brak chiqdi",
		},
	)
	if err != nil {
		t.Fatalf("CustomerRespondDeliveryRequest() error = %v", err)
	}
	if returnAgainst != "MAT-DN-0001" {
		t.Fatalf("expected partial return against MAT-DN-0001, got %q", returnAgainst)
	}
	if returnedQty != 3 {
		t.Fatalf("expected partial return qty 3, got %.2f", returnedQty)
	}
	if detail.Record.Status != "partial" {
		t.Fatalf("expected partial status, got %+v", detail.Record)
	}
	if detail.Record.AcceptedQty != 7 {
		t.Fatalf("expected accepted qty 7, got %+v", detail.Record)
	}
}

func TestCustomerClaimAfterAcceptCreatesReturnDeliveryNote(t *testing.T) {
	var returnedQty float64

	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                "MAT-DN-0001",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-001",
				ItemName:            "Chers",
				Qty:                 10,
				UOM:                 "Nos",
				PostingDate:         "2026-03-15",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "3",
			}, nil
		},
		createPartialDeliveryReturn: func(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, qty float64) (erpnext.DeliveryNoteResult, error) {
			returnedQty = qty
			return erpnext.DeliveryNoteResult{Name: "RET-DN-0002"}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	detail, err := auth.CustomerRespondDeliveryRequest(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		CustomerDeliveryResponseRequest{
			DeliveryNoteID: "MAT-DN-0001",
			Mode:           CustomerDeliveryResponseClaimAfterAccept,
			ReturnedQty:    2,
			Reason:         "Brak chiqdi",
			Comment:        "Ikki dona siniq",
		},
	)
	if err != nil {
		t.Fatalf("CustomerRespondDeliveryRequest() error = %v", err)
	}
	if returnedQty != 2 {
		t.Fatalf("expected claim return qty 2, got %.2f", returnedQty)
	}
	if detail.Record.Status != "partial" {
		t.Fatalf("expected partial status after claim, got %+v", detail.Record)
	}
	if detail.Record.AcceptedQty != 8 {
		t.Fatalf("expected accepted qty 8 after claim, got %+v", detail.Record)
	}
}

func TestCreateWerkaCustomerIssueMapsNegativeStockAndDeletesDraft(t *testing.T) {
	var deletedName string

	stub := &adminSuppliersERPStub{
		getItemsByCodes: func(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error) {
			return []erpnext.Item{{Code: "pista93784", Name: "pista", UOM: "Kg"}}, nil
		},
		searchWarehouses: func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error) {
			return []erpnext.Warehouse{{Name: "Stores - A"}}, nil
		},
		searchCompanies: func(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error) {
			return []erpnext.Company{{Name: "Main Company"}}, nil
		},
		createDraftDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
			return erpnext.DeliveryNoteResult{Name: "MAT-DN-DRAFT-1"}, nil
		},
		submitDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
			return errors.New(`status 417: {"exception":"erpnext.stock.stock_ledger.NegativeStockError"}`)
		},
		deleteDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
			deletedName = name
			return nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.CreateWerkaCustomerIssue(
		context.Background(),
		Principal{Role: RoleWerka, Ref: "werka"},
		"comfi",
		"pista93784",
		9,
	)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
	if deletedName != "MAT-DN-DRAFT-1" {
		t.Fatalf("expected draft cleanup, got %q", deletedName)
	}
}

func TestCreateWerkaCustomerIssueUsesSubmittedRefsDirectly(t *testing.T) {
	var createInput erpnext.CreateDeliveryNoteInput

	stub := &adminSuppliersERPStub{
		getCustomer: func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Customer, error) {
			t.Fatalf("GetCustomer should not be called")
			return erpnext.Customer{}, nil
		},
		listCustomerItems: func(ctx context.Context, baseURL, apiKey, apiSecret, customerRef, query string, limit int) ([]erpnext.Item, error) {
			t.Fatalf("ListCustomerItems should not be called")
			return nil, nil
		},
		getItemsByCodes: func(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error) {
			return []erpnext.Item{{Code: "ITEM-APP-001", Name: "Exact Item", UOM: "Kg"}}, nil
		},
		searchWarehouses: func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error) {
			return []erpnext.Warehouse{{Name: "Stores - A"}}, nil
		},
		searchCompanies: func(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error) {
			return []erpnext.Company{{Name: "Main Company"}}, nil
		},
		createDraftDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
			createInput = input
			return erpnext.DeliveryNoteResult{Name: "MAT-DN-DRAFT-2"}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	record, err := auth.CreateWerkaCustomerIssue(
		context.Background(),
		Principal{Role: RoleWerka, Ref: "werka"},
		"CUST-APP-001",
		"ITEM-APP-001",
		3,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createInput.Customer != "CUST-APP-001" {
		t.Fatalf("expected submitted customer ref to be used directly, got %+v", createInput)
	}
	if createInput.ItemCode != "ITEM-APP-001" || createInput.Qty != 3 {
		t.Fatalf("unexpected create input: %+v", createInput)
	}
	if record.CustomerRef != "CUST-APP-001" || record.CustomerName != "CUST-APP-001" {
		t.Fatalf("unexpected record customer: %+v", record)
	}
}

func TestCustomerCanAddCommentToCustomerDeliveryResultEvent(t *testing.T) {
	var addedDeliveryNoteName string
	stub := &adminSuppliersERPStub{
		getDeliveryNote: func(ctx context.Context, baseURL, apiKey, apiSecret, name string) (erpnext.DeliveryNoteDraft, error) {
			return erpnext.DeliveryNoteDraft{
				Name:                 "MAT-DN-0001",
				Customer:             "CUST-001",
				CustomerName:         "Comfi",
				ItemCode:             "ITEM-001",
				ItemName:             "Chers",
				Qty:                  3,
				UOM:                  "Nos",
				PostingDate:          "2026-03-15",
				DocStatus:            1,
				AccordFlowState:      "1",
				AccordCustomerState:  "2",
				AccordCustomerReason: "Noto'g'ri mahsulot",
			}, nil
		},
		listDeliveryNoteComments: func(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]erpnext.Comment, error) {
			return []erpnext.Comment{}, nil
		},
		addDeliveryNoteComment: func(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
			addedDeliveryNoteName = name
			return nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.AddNotificationComment(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		customerDeliveryResultEventPrefix+"MAT-DN-0001",
		"Men xato ko'ribman, qayta tekshiraman",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addedDeliveryNoteName != "MAT-DN-0001" {
		t.Fatalf("expected delivery note comment target MAT-DN-0001, got %q", addedDeliveryNoteName)
	}
}

func TestCustomerCannotOpenPurchaseReceiptNotificationDetail(t *testing.T) {
	auth := NewERPAuthenticator(
		&adminSuppliersERPStub{},
		"http://erp.test",
		"key",
		"secret",
		"Main - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.NotificationDetail(
		context.Background(),
		Principal{Role: RoleCustomer, Ref: "CUST-001"},
		"MAT-PRE-0001",
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAdminAssignedSupplierItemsReturnsEmptyWhenPermissionDeniedWithoutCache(t *testing.T) {
	stub := &adminSuppliersERPStub{
		getSupplier: func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error) {
			return erpnext.Supplier{ID: id, Name: "Supplier"}, nil
		},
		listAssignedSupplierItems: func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error) {
			return nil, errors.New(`status 403: {"exception":"frappe.exceptions.PermissionError","exc":"check_parent_permission","route":"Item%20Supplier"}`)
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	items, err := auth.AdminAssignedSupplierItems(context.Background(), "SUP-001", 20)
	if err != nil {
		t.Fatalf("AdminAssignedSupplierItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %d", len(items))
	}
}

func TestSupplierItemsUsesLiveERPAssignmentsWhenLocalCacheIsStale(t *testing.T) {
	tempDir := t.TempDir()
	store := NewAdminSupplierStore(filepath.Join(tempDir, "admin-suppliers.json"))
	if err := store.Put("SUP-001", AdminSupplierState{
		AssignmentsConfigured: true,
		AssignedItemCodes:     []string{"ITEM-OLD"},
	}); err != nil {
		t.Fatalf("seed admin supplier state: %v", err)
	}

	stub := &adminSuppliersERPStub{
		listAssignedSupplierItems: func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error) {
			return []erpnext.Item{{Code: "ITEM-NEW", Name: "New Item", UOM: "Nos"}}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		store,
	)

	items, err := auth.SupplierItems(
		context.Background(),
		Principal{Role: RoleSupplier, Ref: "SUP-001"},
		"",
		20,
	)
	if err != nil {
		t.Fatalf("SupplierItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Code != "ITEM-NEW" {
		t.Fatalf("expected ITEM-NEW, got %q", items[0].Code)
	}
}

func TestValidateSupplierItemAllowedUsesLiveERPAssignmentsWhenLocalCacheIsStale(t *testing.T) {
	tempDir := t.TempDir()
	store := NewAdminSupplierStore(filepath.Join(tempDir, "admin-suppliers.json"))
	if err := store.Put("SUP-001", AdminSupplierState{
		AssignmentsConfigured: true,
		AssignedItemCodes:     []string{"ITEM-OLD"},
	}); err != nil {
		t.Fatalf("seed admin supplier state: %v", err)
	}

	stub := &adminSuppliersERPStub{
		listAssignedSupplierItems: func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error) {
			return []erpnext.Item{{Code: "ITEM-NEW", Name: "New Item", UOM: "Nos"}}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		store,
	)

	if err := auth.validateSupplierItemAllowed(context.Background(), "SUP-001", "ITEM-NEW"); err != nil {
		t.Fatalf("validateSupplierItemAllowed() error = %v", err)
	}
}

func TestAdminUpdateSupplierPhoneNormalizesAndPersists(t *testing.T) {
	var updatedPhone string
	var updatedDetails string

	stub := &adminSuppliersERPStub{
		getSupplier: func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error) {
			return erpnext.Supplier{
				ID:      id,
				Name:    "Supplier",
				Details: "Accord Code: 10ABCDEF1234",
			}, nil
		},
		updateSupplierContact: func(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error {
			updatedPhone = phone
			updatedDetails = details
			return nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.AdminUpdateSupplierPhone(context.Background(), "SUP-001", "90 123 45 67")
	if err != nil {
		t.Fatalf("AdminUpdateSupplierPhone() error = %v", err)
	}
	if updatedPhone != "+998901234567" {
		t.Fatalf("expected normalized phone, got %q", updatedPhone)
	}
	if updatedDetails != "Telefon: +998901234567\nAccord Code: 10ABCDEF1234" {
		t.Fatalf("unexpected details payload: %q", updatedDetails)
	}
}

func TestAdminSettingsRoundTripsDefaultUOM(t *testing.T) {
	t.Setenv("ERP_DEFAULT_UOM", "")

	auth := NewERPAuthenticator(
		&adminSuppliersERPStub{},
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	initial := auth.AdminSettings()
	if initial.DefaultUOM != "Kg" {
		t.Fatalf("unexpected initial default UOM: %q", initial.DefaultUOM)
	}

	initial.DefaultUOM = "Lt"
	if err := auth.UpdateAdminSettings(initial); err != nil {
		t.Fatalf("UpdateAdminSettings() error = %v", err)
	}

	updated := auth.AdminSettings()
	if updated.DefaultUOM != "Lt" {
		t.Fatalf("unexpected updated default UOM: %q", updated.DefaultUOM)
	}
}

func TestAdminSupplierSummaryCountsRemovedAsBlockedBucket(t *testing.T) {
	tempDir := t.TempDir()
	store := NewAdminSupplierStore(filepath.Join(tempDir, "admin-suppliers.json"))
	if err := store.Put("SUP-002", AdminSupplierState{Blocked: true}); err != nil {
		t.Fatalf("seed blocked state: %v", err)
	}
	if err := store.Put("SUP-003", AdminSupplierState{Removed: true}); err != nil {
		t.Fatalf("seed removed state: %v", err)
	}

	stub := &adminSuppliersERPStub{
		searchSuppliers: func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error) {
			return []erpnext.Supplier{
				{ID: "SUP-001", Name: "Active"},
				{ID: "SUP-002", Name: "Blocked"},
				{ID: "SUP-003", Name: "Removed"},
			}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		store,
	)

	summary, err := auth.AdminSupplierSummary(context.Background(), 20)
	if err != nil {
		t.Fatalf("AdminSupplierSummary() error = %v", err)
	}
	if summary.TotalSuppliers != 3 || summary.ActiveSuppliers != 1 || summary.BlockedSuppliers != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAdminSearchItemsCachesResolvedWarehouse(t *testing.T) {
	searchWarehouseCalls := 0
	stub := &adminSuppliersERPStub{
		searchWarehouses: func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error) {
			searchWarehouseCalls++
			return []erpnext.Warehouse{{Name: "Stores - A"}}, nil
		},
	}

	auth := NewERPAuthenticator(
		stub,
		"http://erp.test",
		"key",
		"secret",
		"",
		"10",
		"20",
		"",
		"",
		"",
		nil,
		nil,
	)

	_, err := auth.mapSupplierItems(context.Background(), []erpnext.Item{
		{Code: "ITEM-001", Name: "Bolt", UOM: "Nos"},
	})
	if err != nil {
		t.Fatalf("first mapSupplierItems() error = %v", err)
	}
	_, err = auth.mapSupplierItems(context.Background(), []erpnext.Item{
		{Code: "ITEM-002", Name: "Nut", UOM: "Nos"},
	})
	if err != nil {
		t.Fatalf("second mapSupplierItems() error = %v", err)
	}
	if searchWarehouseCalls != 1 {
		t.Fatalf("expected 1 warehouse lookup, got %d", searchWarehouseCalls)
	}
}

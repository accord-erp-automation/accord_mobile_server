package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mobile_server/internal/erpnext"
)

type adminSuppliersERPStub struct {
	searchSuppliers           func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Supplier, error)
	getSupplier               func(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Supplier, error)
	listAssignedSupplierItems func(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]erpnext.Item, error)
	getItemsByCodes           func(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error)
	searchWarehouses          func(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Warehouse, error)
	updateSupplierContact     func(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error
}

func (s *adminSuppliersERPStub) SearchItems(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Item, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) SearchCompanies(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]erpnext.Company, error) {
	return nil, nil
}

func (s *adminSuppliersERPStub) GetCustomer(ctx context.Context, baseURL, apiKey, apiSecret, id string) (erpnext.Customer, error) {
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
	return nil, nil
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

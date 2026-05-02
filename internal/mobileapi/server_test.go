package mobileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

type fakeERPClient struct {
	customers             []erpnext.Customer
	suppliers             []erpnext.Supplier
	items                 []erpnext.Item
	supplierItems         map[string]map[string]bool
	customerItems         map[string]map[string]bool
	uploadedAvatarURL     string
	comments              map[string][]erpnext.Comment
	pendingReceipts       []erpnext.PurchaseReceiptDraft
	supplierReceipts      []erpnext.PurchaseReceiptDraft
	telegramReceipts      []erpnext.PurchaseReceiptDraft
	customerDeliveryNotes []erpnext.DeliveryNoteDraft
	batchCommentKeys      [][]string
	updateRemarksErr      error
	submitDeliveryNoteErr error
	lastSupplierLimit     int
	lastSupplierOffset    int
	lastTelegramLimit     int
	lastTelegramOffset    int
	lastStockEntry        erpnext.CreateStockEntryInput
	lastDeliveryNote      erpnext.CreateDeliveryNoteInput
	lastDeliveryReturn    string
	lastDeliveryReturnQty float64
}

type recordedPushCall struct {
	key   string
	title string
	body  string
	data  map[string]string
}

type recordingPushSender struct {
	calls []recordedPushCall
}

func (r *recordingPushSender) SendToKey(_ context.Context, key, title, body string, data map[string]string) error {
	cloned := map[string]string{}
	for k, v := range data {
		cloned[k] = v
	}
	r.calls = append(r.calls, recordedPushCall{
		key:   key,
		title: title,
		body:  body,
		data:  cloned,
	})
	return nil
}

func (f *fakeERPClient) SearchItems(_ context.Context, _, _, _, query string, limit int) ([]erpnext.Item, error) {
	return filterFakeItems(f.items, query, limit), nil
}

func (f *fakeERPClient) SearchCustomers(_ context.Context, _, _, _, query string, limit int) ([]erpnext.Customer, error) {
	if limit <= 0 || limit > len(f.customers) {
		limit = len(f.customers)
	}
	if query == "" {
		return append([]erpnext.Customer(nil), f.customers[:limit]...), nil
	}
	lowerQuery := strings.ToLower(query)
	result := make([]erpnext.Customer, 0, limit)
	for _, item := range f.customers {
		if strings.Contains(strings.ToLower(item.ID), lowerQuery) ||
			strings.Contains(strings.ToLower(item.Name), lowerQuery) {
			result = append(result, item)
		}
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (f *fakeERPClient) SearchCompanies(_ context.Context, _, _, _ string, _ int) ([]erpnext.Company, error) {
	return []erpnext.Company{{Name: "accord"}}, nil
}

func (f *fakeERPClient) GetCustomer(_ context.Context, _, _, _, id string) (erpnext.Customer, error) {
	for _, item := range f.customers {
		if item.ID == id {
			return item, nil
		}
	}
	return erpnext.Customer{}, nil
}

func (f *fakeERPClient) EnsureCustomer(_ context.Context, _, _, _ string, input erpnext.CreateCustomerInput) (erpnext.Customer, error) {
	return erpnext.Customer{
		ID:    input.Name,
		Name:  input.Name,
		Phone: input.Phone,
	}, nil
}

func (f *fakeERPClient) UpdateCustomerDetails(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func (f *fakeERPClient) UpdateCustomerContact(_ context.Context, _, _, _, _, _, _ string) error {
	return nil
}

func (f *fakeERPClient) SearchSuppliers(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Supplier, error) {
	return f.suppliers, nil
}

func (f *fakeERPClient) GetItemsByCodes(_ context.Context, _, _, _ string, itemCodes []string) ([]erpnext.Item, error) {
	result := make([]erpnext.Item, 0, len(itemCodes))
	for _, code := range itemCodes {
		for _, item := range f.items {
			if item.Code == code {
				result = append(result, item)
				break
			}
		}
	}
	return result, nil
}

func (f *fakeERPClient) CreateItem(_ context.Context, _, _, _ string, input erpnext.CreateItemInput) (erpnext.Item, error) {
	item := erpnext.Item{
		Code: input.Code,
		Name: input.Name,
		UOM:  input.UOM,
	}
	f.items = append(f.items, item)
	return item, nil
}

func (f *fakeERPClient) SearchWarehouses(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Warehouse, error) {
	return []erpnext.Warehouse{{Name: "Stores - A"}}, nil
}

func (f *fakeERPClient) EnsureSupplier(_ context.Context, _, _, _ string, input erpnext.CreateSupplierInput) (erpnext.Supplier, error) {
	return erpnext.Supplier{ID: input.Name, Name: input.Name, Phone: input.Phone}, nil
}

func (f *fakeERPClient) SearchSupplierItems(_ context.Context, _, _, _, _, _ string, _ int) ([]erpnext.Item, error) {
	return f.items, nil
}

func (f *fakeERPClient) ListCustomerItems(_ context.Context, _, _, _, _ string, query string, limit int) ([]erpnext.Item, error) {
	return filterFakeItems(f.items, query, limit), nil
}

func (f *fakeERPClient) GetItemCustomerAssignment(_ context.Context, _, _, _, itemCode string) (erpnext.ItemCustomerAssignment, error) {
	return erpnext.ItemCustomerAssignment{
		Code:         strings.TrimSpace(itemCode),
		CustomerRefs: []string{},
	}, nil
}

func (f *fakeERPClient) ListCustomerDeliveryNotes(_ context.Context, _, _, _, _ string, limit int) ([]erpnext.DeliveryNoteDraft, error) {
	return f.ListCustomerDeliveryNotesPage(context.Background(), "", "", "", "", limit, 0)
}

func (f *fakeERPClient) ListCustomerDeliveryNotesPage(_ context.Context, _, _, _, _ string, limit, offset int) ([]erpnext.DeliveryNoteDraft, error) {
	if f.customerDeliveryNotes != nil {
		return sliceDeliveryNotePage(f.customerDeliveryNotes, limit, offset), nil
	}
	return []erpnext.DeliveryNoteDraft{}, nil
}

func (f *fakeERPClient) GetDeliveryNote(_ context.Context, _, _, _, name string) (erpnext.DeliveryNoteDraft, error) {
	for _, item := range f.customerDeliveryNotes {
		if item.Name == name {
			return item, nil
		}
	}
	return erpnext.DeliveryNoteDraft{}, nil
}

func (f *fakeERPClient) ListDeliveryNoteComments(_ context.Context, _, _, _, name string, _ int) ([]erpnext.Comment, error) {
	return f.comments[strings.TrimSpace(name)], nil
}

func (f *fakeERPClient) ListDeliveryNoteCommentsBatch(_ context.Context, _, _, _ string, names []string, _ int) (map[string][]erpnext.Comment, error) {
	result := make(map[string][]erpnext.Comment, len(names))
	for _, name := range names {
		result[strings.TrimSpace(name)] = f.comments[strings.TrimSpace(name)]
	}
	return result, nil
}

func (f *fakeERPClient) ListAssignedSupplierItems(_ context.Context, _, _, _, supplier string, _ int) ([]erpnext.Item, error) {
	result := make([]erpnext.Item, 0)
	assigned := f.supplierItems[strings.TrimSpace(supplier)]
	for _, item := range f.items {
		if assigned[item.Code] {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeERPClient) AssignSupplierToItem(_ context.Context, _, _, _, itemCode, supplier string) error {
	if f.supplierItems == nil {
		f.supplierItems = map[string]map[string]bool{}
	}
	if f.supplierItems[supplier] == nil {
		f.supplierItems[supplier] = map[string]bool{}
	}
	f.supplierItems[supplier][itemCode] = true
	return nil
}

func (f *fakeERPClient) RemoveSupplierFromItem(_ context.Context, _, _, _, itemCode, supplier string) error {
	if f.supplierItems == nil || f.supplierItems[supplier] == nil {
		return nil
	}
	delete(f.supplierItems[supplier], itemCode)
	return nil
}

func (f *fakeERPClient) AssignCustomerToItem(_ context.Context, _, _, _, itemCode, customerRef string) error {
	if f.customerItems == nil {
		f.customerItems = map[string]map[string]bool{}
	}
	if f.customerItems[customerRef] == nil {
		f.customerItems[customerRef] = map[string]bool{}
	}
	f.customerItems[customerRef][itemCode] = true
	return nil
}

func (f *fakeERPClient) RemoveCustomerFromItem(_ context.Context, _, _, _, itemCode, customerRef string) error {
	if f.customerItems == nil || f.customerItems[customerRef] == nil {
		return nil
	}
	delete(f.customerItems[customerRef], itemCode)
	return nil
}

func (f *fakeERPClient) GetSupplier(_ context.Context, _, _, _, id string) (erpnext.Supplier, error) {
	for _, item := range f.suppliers {
		if item.ID == id {
			return item, nil
		}
	}
	return erpnext.Supplier{}, nil
}

func (f *fakeERPClient) UpdateSupplierDetails(_ context.Context, _, _, _, id, details string) error {
	for index, item := range f.suppliers {
		if item.ID == id {
			item.Details = details
			f.suppliers[index] = item
			return nil
		}
	}
	return nil
}

func (f *fakeERPClient) UpdateSupplierContact(_ context.Context, _, _, _, id, phone, details string) error {
	for index, item := range f.suppliers {
		if item.ID == id {
			item.Phone = phone
			item.Details = details
			f.suppliers[index] = item
			return nil
		}
	}
	return nil
}

func (f *fakeERPClient) ListPendingPurchaseReceipts(_ context.Context, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return f.ListPendingPurchaseReceiptsPage(context.Background(), "", "", "", 0, 0)
}

func (f *fakeERPClient) ListPendingPurchaseReceiptsPage(_ context.Context, _, _, _ string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	if f.pendingReceipts != nil {
		return sliceReceiptPage(f.pendingReceipts, limit, offset), nil
	}
	return nil, nil
}

func (f *fakeERPClient) ListTelegramPurchaseReceipts(_ context.Context, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return f.ListTelegramPurchaseReceiptsPage(context.Background(), "", "", "", 0, 0)
}

func (f *fakeERPClient) ListTelegramPurchaseReceiptsPage(_ context.Context, _, _, _ string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	f.lastTelegramLimit = limit
	f.lastTelegramOffset = offset
	if f.telegramReceipts != nil {
		return sliceReceiptPage(f.telegramReceipts, limit, offset), nil
	}
	return sliceReceiptPage([]erpnext.PurchaseReceiptDraft{
		{
			Name:                 "MAT-PRE-0001",
			SupplierName:         "Abdulloh",
			SupplierDeliveryNote: "TG:+998900000000|25",
			ItemCode:             "ITEM-001",
			ItemName:             "Rice",
			Qty:                  25,
			UOM:                  "Kg",
			PostingDate:          "2026-03-10",
		},
	}, limit, offset), nil
}

func (f *fakeERPClient) ListSupplierPurchaseReceipts(_ context.Context, _, _, _, supplier string, limit int) ([]erpnext.PurchaseReceiptDraft, error) {
	return f.ListSupplierPurchaseReceiptsPage(context.Background(), "", "", "", supplier, limit, 0)
}

func (f *fakeERPClient) ListSupplierPurchaseReceiptsPage(_ context.Context, _, _, _, _ string, limit, offset int) ([]erpnext.PurchaseReceiptDraft, error) {
	f.lastSupplierLimit = limit
	f.lastSupplierOffset = offset
	if f.supplierReceipts != nil {
		return sliceReceiptPage(f.supplierReceipts, limit, offset), nil
	}
	return sliceReceiptPage([]erpnext.PurchaseReceiptDraft{
		{
			Name:                 "MAT-PRE-0001",
			Supplier:             "SUP-001",
			SupplierName:         "Abdulloh",
			SupplierDeliveryNote: "TG:+998900000000|25",
			ItemCode:             "ITEM-001",
			ItemName:             "Rice",
			Qty:                  25,
			UOM:                  "Kg",
			PostingDate:          "2026-03-10",
		},
	}, limit, offset), nil
}

func (f *fakeERPClient) GetPurchaseReceipt(_ context.Context, _, _, _, name string) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{
		Name:                 name,
		Supplier:             "SUP-001",
		SupplierName:         "Abdulloh",
		SupplierDeliveryNote: "TG:+998900000000|25",
		ItemCode:             "ITEM-001",
		ItemName:             "Rice",
		Qty:                  25,
		UOM:                  "Kg",
		PostingDate:          "2026-03-10",
	}, nil
}

func (f *fakeERPClient) ListPurchaseReceiptComments(_ context.Context, _, _, _, name string, _ int) ([]erpnext.Comment, error) {
	return append([]erpnext.Comment(nil), f.comments[name]...), nil
}

func (f *fakeERPClient) ListPurchaseReceiptCommentsBatch(_ context.Context, _, _, _ string, names []string, _ int) (map[string][]erpnext.Comment, error) {
	f.batchCommentKeys = append(f.batchCommentKeys, append([]string(nil), names...))
	result := make(map[string][]erpnext.Comment, len(names))
	for _, name := range names {
		result[name] = append([]erpnext.Comment(nil), f.comments[name]...)
	}
	return result, nil
}

func (f *fakeERPClient) AddPurchaseReceiptComment(_ context.Context, _, _, _, name, content string) error {
	if f.comments == nil {
		f.comments = map[string][]erpnext.Comment{}
	}
	f.comments[name] = append(f.comments[name], erpnext.Comment{
		ID:        "COMM-001",
		Content:   content,
		CreatedAt: "2026-03-11 10:00:00",
	})
	return nil
}

func (f *fakeERPClient) UpdatePurchaseReceiptRemarks(_ context.Context, _, _, _, name, remarks string) error {
	return f.updateRemarksErr
}

func (f *fakeERPClient) CreateDraftPurchaseReceipt(_ context.Context, _, _, _ string, _ erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{}, nil
}

func (f *fakeERPClient) CreateAndSubmitStockEntry(_ context.Context, _, _, _ string, input erpnext.CreateStockEntryInput) (erpnext.StockEntryResult, error) {
	f.lastStockEntry = input
	return erpnext.StockEntryResult{Name: "STE-0001"}, nil
}

func (f *fakeERPClient) CreateAndSubmitDeliveryNote(_ context.Context, _, _, _ string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
	f.lastDeliveryNote = input
	name := "MAT-DN-0001"
	f.customerDeliveryNotes = append([]erpnext.DeliveryNoteDraft{{
		Name:         name,
		Customer:     input.Customer,
		CustomerName: input.Customer,
		ItemCode:     input.ItemCode,
		ItemName:     input.ItemCode,
		Qty:          input.Qty,
		UOM:          input.UOM,
		PostingDate:  "2026-03-14",
		Status:       "Submitted",
		DocStatus:    1,
		Remarks:      input.Remarks,
	}}, f.customerDeliveryNotes...)
	return erpnext.DeliveryNoteResult{Name: name}, nil
}

func (f *fakeERPClient) CreateDraftDeliveryNote(_ context.Context, _, _, _ string, input erpnext.CreateDeliveryNoteInput) (erpnext.DeliveryNoteResult, error) {
	f.lastDeliveryNote = input
	name := "MAT-DN-0001"
	f.customerDeliveryNotes = append([]erpnext.DeliveryNoteDraft{{
		Name:         name,
		Customer:     input.Customer,
		CustomerName: input.Customer,
		ItemCode:     input.ItemCode,
		ItemName:     input.ItemCode,
		Qty:          input.Qty,
		UOM:          input.UOM,
		PostingDate:  "2026-03-14",
		Status:       "Draft",
		DocStatus:    0,
		Remarks:      input.Remarks,
	}}, f.customerDeliveryNotes...)
	return erpnext.DeliveryNoteResult{Name: name}, nil
}

func (f *fakeERPClient) CreateAndSubmitDeliveryNoteReturn(_ context.Context, _, _, _, sourceName string) (erpnext.DeliveryNoteResult, error) {
	f.lastDeliveryReturn = sourceName
	f.lastDeliveryReturnQty = 0
	name := "RET-DN-0001"
	f.customerDeliveryNotes = append([]erpnext.DeliveryNoteDraft{{
		Name:         name,
		Customer:     "comfi",
		CustomerName: "comfi",
		ItemCode:     "pista93784",
		ItemName:     "pista",
		Qty:          -1,
		UOM:          "Kg",
		PostingDate:  "2026-03-23",
		Status:       "Return",
		DocStatus:    1,
		Remarks:      sourceName,
	}}, f.customerDeliveryNotes...)
	return erpnext.DeliveryNoteResult{Name: name}, nil
}

func (f *fakeERPClient) CreateAndSubmitPartialDeliveryNoteReturn(_ context.Context, _, _, _, sourceName string, returnedQty float64) (erpnext.DeliveryNoteResult, error) {
	f.lastDeliveryReturn = sourceName
	f.lastDeliveryReturnQty = returnedQty
	name := "RET-DN-0001"
	f.customerDeliveryNotes = append([]erpnext.DeliveryNoteDraft{{
		Name:         name,
		Customer:     "comfi",
		CustomerName: "comfi",
		ItemCode:     "pista93784",
		ItemName:     "pista",
		Qty:          -returnedQty,
		UOM:          "Kg",
		PostingDate:  "2026-03-23",
		Status:       "Return",
		DocStatus:    1,
		Remarks:      sourceName,
	}}, f.customerDeliveryNotes...)
	return erpnext.DeliveryNoteResult{Name: name}, nil
}

func (f *fakeERPClient) SubmitDeliveryNote(_ context.Context, _, _, _, name string) error {
	if f.submitDeliveryNoteErr != nil {
		return f.submitDeliveryNoteErr
	}
	for index, item := range f.customerDeliveryNotes {
		if item.Name == name {
			item.DocStatus = 1
			item.Status = "Submitted"
			f.customerDeliveryNotes[index] = item
			return nil
		}
	}
	return nil
}

func (f *fakeERPClient) EnsureDeliveryNoteStateFields(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeERPClient) UpdateDeliveryNoteState(_ context.Context, _, _, _, name string, update erpnext.DeliveryNoteStateUpdate) error {
	for index, item := range f.customerDeliveryNotes {
		if item.Name == name {
			item.AccordFlowState = update.FlowState
			item.AccordCustomerState = update.CustomerState
			item.AccordCustomerReason = update.CustomerReason
			item.AccordDeliveryActor = update.DeliveryActor
			item.AccordUIStatus = update.UIStatus
			f.customerDeliveryNotes[index] = item
			return nil
		}
	}
	return nil
}

func (f *fakeERPClient) UpdateDeliveryNoteRemarks(_ context.Context, _, _, _, name, remarks string) error {
	for index, item := range f.customerDeliveryNotes {
		if item.Name == name {
			item.Remarks = remarks
			f.customerDeliveryNotes[index] = item
			return nil
		}
	}
	return nil
}

func (f *fakeERPClient) AddDeliveryNoteComment(_ context.Context, _, _, _, name, content string) error {
	if f.comments == nil {
		f.comments = map[string][]erpnext.Comment{}
	}
	name = strings.TrimSpace(name)
	f.comments[name] = append(f.comments[name], erpnext.Comment{
		ID:        fmt.Sprintf("dn-comment-%d", len(f.comments[name])+1),
		Content:   content,
		CreatedAt: "2026-03-14 10:00:00",
	})
	return nil
}

func (f *fakeERPClient) DeleteDeliveryNote(_ context.Context, _, _, _, name string) error {
	filtered := f.customerDeliveryNotes[:0]
	for _, item := range f.customerDeliveryNotes {
		if item.Name == name {
			continue
		}
		filtered = append(filtered, item)
	}
	f.customerDeliveryNotes = filtered
	return nil
}

func (f *fakeERPClient) ConfirmAndSubmitPurchaseReceipt(_ context.Context, _, _, _, _ string, _, _ float64, _, _ string) (erpnext.PurchaseReceiptSubmissionResult, error) {
	return erpnext.PurchaseReceiptSubmissionResult{}, nil
}

func (f *fakeERPClient) UploadSupplierImage(_ context.Context, _, _, _, supplierID, _, _ string, _ []byte) (string, error) {
	fileURL := "/files/" + supplierID + "-avatar.png"
	f.uploadedAvatarURL = fileURL
	for index, item := range f.suppliers {
		if item.ID == supplierID {
			item.Image = fileURL
			f.suppliers[index] = item
			break
		}
	}
	return fileURL, nil
}

func (f *fakeERPClient) DownloadFile(_ context.Context, _, _, _, fileURL string) (string, []byte, error) {
	return "image/png", []byte("pngdata"), nil
}

func filterFakeItems(items []erpnext.Item, query string, limit int) []erpnext.Item {
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	if query == "" {
		return append([]erpnext.Item(nil), items[:limit]...)
	}
	lowerQuery := strings.ToLower(query)
	result := make([]erpnext.Item, 0, limit)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Code), lowerQuery) ||
			strings.Contains(strings.ToLower(item.Name), lowerQuery) {
			result = append(result, item)
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

func sliceReceiptPage(items []erpnext.PurchaseReceiptDraft, limit, offset int) []erpnext.PurchaseReceiptDraft {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []erpnext.PurchaseReceiptDraft{}
	}
	if limit <= 0 || offset+limit > len(items) {
		limit = len(items) - offset
	}
	return append([]erpnext.PurchaseReceiptDraft(nil), items[offset:offset+limit]...)
}

func sliceDeliveryNotePage(items []erpnext.DeliveryNoteDraft, limit, offset int) []erpnext.DeliveryNoteDraft {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []erpnext.DeliveryNoteDraft{}
	}
	if limit <= 0 || offset+limit > len(items) {
		limit = len(items) - offset
	}
	return append([]erpnext.DeliveryNoteDraft(nil), items[offset:offset+limit]...)
}

func TestServerLoginAndMeFlow(t *testing.T) {
	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   "SUP-001",
		Name:  "Abdulloh",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("failed to generate access credentials: %v", err)
	}

	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			suppliers: []erpnext.Supplier{
				{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567"},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body, _ := json.Marshal(LoginRequest{
		Phone: "+998901234567",
		Code:  creds.Code,
	})
	resp, err := http.Post(ts.URL+"/v1/mobile/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status: %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("expected token")
	}
	if loginResp.Profile.Role != RoleSupplier || loginResp.Profile.DisplayName != "Abdulloh" {
		t.Fatalf("unexpected profile: %+v", loginResp.Profile)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me request failed: %v", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected me status: %d", meResp.StatusCode)
	}
}

func TestServerLogoutInvalidatesSession(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleSupplier, DisplayName: "Abdulloh"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/mobile/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("unexpected logout status: %d", logoutResp.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", meResp.Code)
	}
}

func TestServerProfileUpdateAndAvatarFlow(t *testing.T) {
	fakeERP := &fakeERPClient{
		suppliers: []erpnext.Supplier{
			{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567", Image: "/files/original.png"},
		},
	}
	profiles := NewProfileStore(t.TempDir() + "/profile_prefs.json")
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		profiles,
		nil,
	))
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   "SUP-001",
		Name:  "Abdulloh",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("failed to generate credentials: %v", err)
	}

	loginBody, _ := json.Marshal(LoginRequest{
		Phone: "+998901234567",
		Code:  creds.Code,
	})
	loginResp, err := http.Post(ts.URL+"/v1/mobile/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginPayload LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginPayload); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	updateReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/v1/mobile/profile", bytes.NewReader([]byte(`{"nickname":"Alias"}`)))
	updateReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("profile update request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected profile update status: %d", updateResp.StatusCode)
	}

	var updated Principal
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode profile update response: %v", err)
	}
	if updated.DisplayName != "Alias" {
		t.Fatalf("unexpected nickname after update: %+v", updated)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := part.Write([]byte("pngdata")); err != nil {
		t.Fatalf("failed to write avatar payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	avatarReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/mobile/profile/avatar", &body)
	avatarReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	avatarReq.Header.Set("Content-Type", writer.FormDataContentType())
	avatarResp, err := http.DefaultClient.Do(avatarReq)
	if err != nil {
		t.Fatalf("avatar request failed: %v", err)
	}
	defer avatarResp.Body.Close()
	if avatarResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected avatar status: %d", avatarResp.StatusCode)
	}

	var avatarPrincipal Principal
	if err := json.NewDecoder(avatarResp.Body).Decode(&avatarPrincipal); err != nil {
		t.Fatalf("failed to decode avatar response: %v", err)
	}
	expectedAvatarURL := ts.URL + "/v1/mobile/profile/avatar/view?token=" + loginPayload.Token
	if avatarPrincipal.AvatarURL != expectedAvatarURL {
		t.Fatalf("unexpected avatar url: %+v", avatarPrincipal)
	}
	if fakeERP.uploadedAvatarURL == "" {
		t.Fatal("expected fake ERP upload to run")
	}
}

func TestServerAdminSupplierManagementFlow(t *testing.T) {
	adminStore := NewAdminSupplierStore(t.TempDir() + "/admin_suppliers.json")
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			suppliers: []erpnext.Supplier{
				{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567"},
				{ID: "SUP-002", Name: "Begzod", Phone: "+998901234568"},
				{ID: "SUP-003", Name: "Dilshod", Phone: "+998901234569"},
			},
			customers: []erpnext.Customer{
				{ID: "CUS-001", Name: "Customer One", Phone: "+998901111111"},
				{ID: "CUS-002", Name: "Customer Two", Phone: "+998901111112"},
				{ID: "CUS-003", Name: "Customer Three", Phone: "+998901111113"},
			},
			items: []erpnext.Item{
				{Code: "ITEM-001", Name: "Rice", UOM: "Kg"},
				{Code: "ITEM-002", Name: "Oil", UOM: "L"},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		adminStore,
	))
	token, err := server.sessions.Create(Principal{Role: RoleAdmin, DisplayName: "Admin"})
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/suppliers", nil)
	pageReq.Header.Set("Authorization", "Bearer "+token)
	pageResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("unexpected suppliers page status: %d", pageResp.Code)
	}

	var page AdminSuppliersPage
	if err := json.NewDecoder(pageResp.Body).Decode(&page); err != nil {
		t.Fatalf("failed to decode suppliers page: %v", err)
	}
	if page.Summary.TotalSuppliers == 0 || len(page.Suppliers) == 0 {
		t.Fatalf("unexpected suppliers page payload: %+v", page)
	}

	supplierListReq := httptest.NewRequest(
		http.MethodGet,
		"/v1/mobile/admin/suppliers/list?limit=2&offset=1",
		nil,
	)
	supplierListReq.Header.Set("Authorization", "Bearer "+token)
	supplierListResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(supplierListResp, supplierListReq)
	if supplierListResp.Code != http.StatusOK {
		t.Fatalf("unexpected supplier list status: %d", supplierListResp.Code)
	}
	var supplierList []AdminSupplier
	if err := json.NewDecoder(supplierListResp.Body).Decode(&supplierList); err != nil {
		t.Fatalf("failed to decode supplier list: %v", err)
	}
	if len(supplierList) != 2 {
		t.Fatalf("unexpected supplier list length: %d", len(supplierList))
	}

	customerListReq := httptest.NewRequest(
		http.MethodGet,
		"/v1/mobile/admin/customers/list?limit=2&offset=1",
		nil,
	)
	customerListReq.Header.Set("Authorization", "Bearer "+token)
	customerListResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(customerListResp, customerListReq)
	if customerListResp.Code != http.StatusOK {
		t.Fatalf("unexpected customer list status: %d", customerListResp.Code)
	}
	var customerList []CustomerDirectoryEntry
	if err := json.NewDecoder(customerListResp.Body).Decode(&customerList); err != nil {
		t.Fatalf("failed to decode customer list: %v", err)
	}
	if len(customerList) != 2 {
		t.Fatalf("unexpected customer list length: %d", len(customerList))
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/suppliers/summary", nil)
	summaryReq.Header.Set("Authorization", "Bearer "+token)
	summaryResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(summaryResp, summaryReq)
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("unexpected summary status: %d", summaryResp.Code)
	}

	statusReq := httptest.NewRequest(
		http.MethodPut,
		"/v1/mobile/admin/suppliers/status?ref=SUP-001",
		bytes.NewReader([]byte(`{"blocked":true}`)),
	)
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusReq.Header.Set("Content-Type", "application/json")
	statusResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("unexpected status update code: %d", statusResp.Code)
	}

	itemsReq := httptest.NewRequest(
		http.MethodPut,
		"/v1/mobile/admin/suppliers/items?ref=SUP-001",
		bytes.NewReader([]byte(`{"item_codes":["ITEM-001","ITEM-002"]}`)),
	)
	itemsReq.Header.Set("Authorization", "Bearer "+token)
	itemsReq.Header.Set("Content-Type", "application/json")
	itemsResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(itemsResp, itemsReq)
	if itemsResp.Code != http.StatusOK {
		t.Fatalf("unexpected item update code: %d", itemsResp.Code)
	}

	phoneReq := httptest.NewRequest(
		http.MethodPut,
		"/v1/mobile/admin/suppliers/phone?ref=SUP-001",
		bytes.NewReader([]byte(`{"phone":"+998909876543"}`)),
	)
	phoneReq.Header.Set("Authorization", "Bearer "+token)
	phoneReq.Header.Set("Content-Type", "application/json")
	phoneResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(phoneResp, phoneReq)
	if phoneResp.Code != http.StatusOK {
		t.Fatalf("unexpected phone update code: %d", phoneResp.Code)
	}

	codeReq := httptest.NewRequest(http.MethodPost, "/v1/mobile/admin/suppliers/code/regenerate?ref=SUP-001", nil)
	codeReq.Header.Set("Authorization", "Bearer "+token)
	codeResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(codeResp, codeReq)
	if codeResp.Code != http.StatusOK {
		t.Fatalf("unexpected code regenerate status: %d", codeResp.Code)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/suppliers/detail?ref=SUP-001", nil)
	detailReq.Header.Set("Authorization", "Bearer "+token)
	detailResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected detail status: %d", detailResp.Code)
	}

	var detail AdminSupplierDetail
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode detail: %v", err)
	}
	if !detail.Blocked {
		t.Fatalf("expected supplier to be blocked: %+v", detail)
	}
	if detail.Phone != "+998909876543" {
		t.Fatalf("expected updated phone, got %+v", detail)
	}
	if len(detail.AssignedItems) != 2 {
		t.Fatalf("expected 2 assigned items, got %+v", detail.AssignedItems)
	}
	if detail.Code == "" {
		t.Fatalf("expected regenerated code, got %+v", detail)
	}

	createItemReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/admin/items",
		bytes.NewReader([]byte(`{"code":"ITEM-003","name":"Flour","uom":"Kg"}`)),
	)
	createItemReq.Header.Set("Authorization", "Bearer "+token)
	createItemReq.Header.Set("Content-Type", "application/json")
	createItemResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(createItemResp, createItemReq)
	if createItemResp.Code != http.StatusOK {
		t.Fatalf("unexpected item create status: %d", createItemResp.Code)
	}

	removeReq := httptest.NewRequest(http.MethodDelete, "/v1/mobile/admin/suppliers/remove?ref=SUP-001", nil)
	removeReq.Header.Set("Authorization", "Bearer "+token)
	removeResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(removeResp, removeReq)
	if removeResp.Code != http.StatusOK {
		t.Fatalf("unexpected remove status: %d", removeResp.Code)
	}

	inactiveReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/suppliers/inactive", nil)
	inactiveReq.Header.Set("Authorization", "Bearer "+token)
	inactiveResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(inactiveResp, inactiveReq)
	if inactiveResp.Code != http.StatusOK {
		t.Fatalf("unexpected inactive list status: %d", inactiveResp.Code)
	}

	restoreReq := httptest.NewRequest(http.MethodPost, "/v1/mobile/admin/suppliers/restore?ref=SUP-001", nil)
	restoreReq.Header.Set("Authorization", "Bearer "+token)
	restoreResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(restoreResp, restoreReq)
	if restoreResp.Code != http.StatusOK {
		t.Fatalf("unexpected restore status: %d", restoreResp.Code)
	}
}

func TestServerWerkaHistoryFlow(t *testing.T) {
	fakeERP := &fakeERPClient{
		telegramReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|25",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  25,
				UOM:                  "Kg",
				PostingDate:          "2026-03-10",
			},
			{
				Name:                 "MAT-PRE-0002",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|25",
				ItemCode:             "ITEM-002",
				ItemName:             "Oil",
				Qty:                  25,
				UOM:                  "Kg",
				PostingDate:          "2026-03-10",
				Remarks:              "Accord Qabul: 20 Kg\nAccord Qaytarildi: 5 Kg",
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleWerka, DisplayName: "Werka"})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected werka history status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode werka history: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected werka history items")
	}
	if len(fakeERP.batchCommentKeys) != 1 {
		t.Fatalf("expected 1 batch comment call, got %d", len(fakeERP.batchCommentKeys))
	}
	if len(fakeERP.batchCommentKeys[0]) != 1 || fakeERP.batchCommentKeys[0][0] != "MAT-PRE-0002" {
		t.Fatalf("unexpected batch comment names: %+v", fakeERP.batchCommentKeys)
	}
}

func TestServerWerkaPendingIncludesDraftReceipts(t *testing.T) {
	fakeERP := &fakeERPClient{
		pendingReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|2",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  2,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Status:               "Draft",
				DocStatus:            0,
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleWerka, DisplayName: "Werka"})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected werka pending status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode werka pending: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending item, got %+v", items)
	}
	if items[0].ID != "MAT-PRE-0001" || items[0].Status != "draft" {
		t.Fatalf("unexpected pending item payload: %+v", items[0])
	}
}

func TestServerWerkaPendingSkipsStaleProcessedDrafts(t *testing.T) {
	fakeERP := &fakeERPClient{
		pendingReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000:20260311120000:3.0000",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  3,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Status:               "Draft",
				DocStatus:            0,
				Remarks:              "Accord Qabul: 1.0000 Kg\nAccord Qaytarildi: 2.0000 Kg",
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleWerka, DisplayName: "Werka"})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected werka pending status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode werka pending: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected stale processed draft to be hidden, got %+v", items)
	}
}

func TestServerWerkaSummarySeparatesReturnedFromConfirmed(t *testing.T) {
	fakeERP := &fakeERPClient{
		telegramReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|2",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  2,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Status:               "Draft",
				DocStatus:            0,
			},
			{
				Name:                 "MAT-PRE-0002",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|3",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  3,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				DocStatus:            1,
			},
			{
				Name:                 "MAT-PRE-0003",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000:20260311120000:4.0000",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  1,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Remarks:              "Accord Qabul: 1.0000 Kg\nAccord Qaytarildi: 3.0000 Kg",
				DocStatus:            1,
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleWerka, DisplayName: "Werka"})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected werka summary status: %d", resp.Code)
	}

	var summary WerkaHomeSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode werka summary: %v", err)
	}
	if summary.PendingCount != 1 || summary.ConfirmedCount != 1 || summary.ReturnedCount != 1 {
		t.Fatalf("unexpected werka summary: %+v", summary)
	}
}

func TestServerWerkaSummaryIncludesCustomerDeliveryNotes(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			telegramReceipts: []erpnext.PurchaseReceiptDraft{},
			customers: []erpnext.Customer{
				{ID: "CUST-001", Name: "Comfi"},
			},
			customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
				{
					Name:                "MAT-DN-1001",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-001",
					ItemName:            "Chers",
					Qty:                 3,
					UOM:                 "Nos",
					PostingDate:         "2026-03-17",
					Modified:            "2026-03-17 10:00:00",
					DocStatus:           1,
					Status:              "To Bill",
					AccordFlowState:     "1",
					AccordCustomerState: "0",
				},
				{
					Name:                "MAT-DN-1002",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-002",
					ItemName:            "Test",
					Qty:                 5,
					UOM:                 "Kg",
					PostingDate:         "2026-03-17",
					Modified:            "2026-03-17 10:05:00",
					DocStatus:           1,
					Status:              "To Bill",
					AccordFlowState:     "1",
					AccordCustomerState: "3",
					AccordDeliveryActor: "1",
				},
				{
					Name:                 "MAT-DN-1003",
					Customer:             "CUST-001",
					CustomerName:         "Comfi",
					ItemCode:             "ITEM-003",
					ItemName:             "Reject",
					Qty:                  2,
					UOM:                  "Nos",
					PostingDate:          "2026-03-17",
					Modified:             "2026-03-17 10:10:00",
					DocStatus:            1,
					Status:               "To Bill",
					AccordFlowState:      "1",
					AccordCustomerState:  "2",
					AccordCustomerReason: "xato",
				},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleWerka,
		DisplayName: "Werka",
		Ref:         "werka",
	})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var summary WerkaHomeSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary failed: %v", err)
	}
	if summary.PendingCount != 1 || summary.ConfirmedCount != 1 || summary.ReturnedCount != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestServerWerkaStatusDetailsFiltersBySupplierAndKind(t *testing.T) {
	fakeERP := &fakeERPClient{
		telegramReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000:20260311120000:2.0000",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  2,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Status:               "Draft",
				DocStatus:            0,
			},
			{
				Name:                 "MAT-PRE-0002",
				Supplier:             "SUP-002",
				SupplierName:         "Ali",
				SupplierDeliveryNote: "TG:+998900000001:20260311120000:5.0000",
				ItemCode:             "ITEM-002",
				ItemName:             "Oil",
				Qty:                  5,
				UOM:                  "Kg",
				PostingDate:          "2026-03-11",
				Status:               "Draft",
				DocStatus:            0,
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleWerka, DisplayName: "Werka"})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/status-details?kind=pending&supplier_ref=SUP-001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected werka status details status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode werka status details: %v", err)
	}
	if len(items) != 1 || items[0].ID != "MAT-PRE-0001" {
		t.Fatalf("unexpected werka status details: %+v", items)
	}
}

func TestServerSupplierHistorySkipsCommentBatchForCleanRecords(t *testing.T) {
	fakeERP := &fakeERPClient{
		supplierReceipts: []erpnext.PurchaseReceiptDraft{
			{
				Name:                 "MAT-PRE-0001",
				Supplier:             "SUP-001",
				SupplierName:         "Abdulloh",
				SupplierDeliveryNote: "TG:+998900000000|25",
				ItemCode:             "ITEM-001",
				ItemName:             "Rice",
				Qty:                  25,
				UOM:                  "Kg",
				PostingDate:          "2026-03-10",
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleSupplier,
		DisplayName: "Abdulloh",
		Ref:         "SUP-001",
	})
	if err != nil {
		t.Fatalf("failed to create supplier session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/supplier/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected supplier history status: %d", resp.Code)
	}
	if len(fakeERP.batchCommentKeys) != 0 {
		t.Fatalf("expected no batch comment calls, got %+v", fakeERP.batchCommentKeys)
	}
}

func TestServerSupplierHistoryReturnsCanonicalFullList(t *testing.T) {
	receipts := make([]erpnext.PurchaseReceiptDraft, 0, 120)
	for index := 0; index < 120; index++ {
		receipts = append(receipts, erpnext.PurchaseReceiptDraft{
			Name:                 fmt.Sprintf("MAT-PRE-%04d", index+1),
			Supplier:             "SUP-001",
			SupplierName:         "Abdulloh",
			SupplierDeliveryNote: fmt.Sprintf("TG:+998900000000|%d", index+1),
			ItemCode:             fmt.Sprintf("ITEM-%03d", index+1),
			ItemName:             fmt.Sprintf("Item %03d", index+1),
			Qty:                  1,
			UOM:                  "Kg",
			PostingDate:          "2026-03-20",
		})
	}
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{supplierReceipts: receipts},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleSupplier,
		DisplayName: "Abdulloh",
		Ref:         "SUP-001",
	})
	if err != nil {
		t.Fatalf("failed to create supplier session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/supplier/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected supplier history status: %d body=%s", resp.Code, resp.Body.String())
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode supplier history: %v", err)
	}
	if len(items) != 120 {
		t.Fatalf("expected 120 supplier history items, got %d", len(items))
	}
}

func TestServerNotificationDetailAndCommentFlow(t *testing.T) {
	fakeERP := &fakeERPClient{
		comments: map[string][]erpnext.Comment{
			"MAT-PRE-0001": {
				{ID: "COMM-0001", Content: "Tizim\nQisman olindi.", CreatedAt: "2026-03-11 09:00:00"},
			},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleSupplier,
		DisplayName: "Abdulloh",
		Ref:         "SUP-001",
	})
	if err != nil {
		t.Fatalf("failed to create supplier session: %v", err)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/notifications/detail?receipt_id=MAT-PRE-0001", nil)
	detailReq.Header.Set("Authorization", "Bearer "+token)
	detailResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected notification detail status: %d", detailResp.Code)
	}

	commentReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/notifications/comments?receipt_id=MAT-PRE-0001",
		bytes.NewReader([]byte(`{"message":"Qaytgan 1 kgni ko‘rdim"}`)),
	)
	commentReq.Header.Set("Authorization", "Bearer "+token)
	commentReq.Header.Set("Content-Type", "application/json")
	commentResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(commentResp, commentReq)
	if commentResp.Code != http.StatusOK {
		t.Fatalf("unexpected notification comment status: %d", commentResp.Code)
	}

	var detail NotificationDetail
	if err := json.NewDecoder(commentResp.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode notification detail: %v", err)
	}
	if len(detail.Comments) < 2 {
		t.Fatalf("expected comments to grow, got %+v", detail.Comments)
	}
}

func TestServerWerkaCustomerIssueBatchCreate(t *testing.T) {
	fakeERP := &fakeERPClient{
		items: []erpnext.Item{
			{Code: "ITEM-001", Name: "Item 001", UOM: "Kg"},
		},
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	pushSender := &recordingPushSender{}
	server.sender = pushSender
	token, err := server.sessions.Create(Principal{
		Role:        RoleWerka,
		DisplayName: "Werka",
		Ref:         "werka",
	})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/werka/customer-issue/batch-create",
		bytes.NewReader([]byte(`{"client_batch_id":"batch-1","lines":[{"customer_ref":"CUST-001","item_code":"ITEM-001","qty":2}]}`)),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected batch create status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload WerkaCustomerIssueBatchResult
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode batch create response: %v", err)
	}
	if payload.ClientBatchID != "batch-1" {
		t.Fatalf("unexpected batch id: %+v", payload)
	}
	if len(payload.Created) != 1 || payload.Created[0].Record == nil {
		t.Fatalf("expected one created record, got %+v", payload)
	}
	if payload.Created[0].LineIndex != 0 {
		t.Fatalf("unexpected line index: %+v", payload.Created[0])
	}
	if len(payload.Failed) != 0 {
		t.Fatalf("expected no failed lines, got %+v", payload.Failed)
	}
	if len(pushSender.calls) != 1 {
		t.Fatalf("expected one push call, got %+v", pushSender.calls)
	}
}

func TestServerCustomerHistoryReturnsCanonicalFullList(t *testing.T) {
	notes := make([]erpnext.DeliveryNoteDraft, 0, 150)
	for index := 0; index < 150; index++ {
		notes = append(notes, erpnext.DeliveryNoteDraft{
			Name:                 fmt.Sprintf("MAT-DN-%04d", index+1),
			Customer:             "CUST-001",
			CustomerName:         "Comfi",
			ItemCode:             fmt.Sprintf("ITEM-%03d", index+1),
			ItemName:             fmt.Sprintf("Item %03d", index+1),
			Qty:                  1,
			UOM:                  "Kg",
			PostingDate:          "2026-03-20",
			Modified:             fmt.Sprintf("2026-03-20 10:%02d:00", index%60),
			DocStatus:            1,
			AccordFlowState:      "1",
			AccordCustomerState:  "0",
			AccordCustomerReason: "",
		})
	}
	fakeERP := &fakeERPClient{
		customerDeliveryNotes: notes,
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleCustomer,
		DisplayName: "Comfi",
		Ref:         "CUST-001",
	})
	if err != nil {
		t.Fatalf("failed to create customer session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/customer/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected customer history status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode customer history: %v", err)
	}
	if len(items) != 150 {
		t.Fatalf("expected 150 customer history items, got %d", len(items))
	}
}

func TestServerSupplierAcknowledgmentCommentSucceedsWhenRemarksBackfillFails(t *testing.T) {
	fakeERP := &fakeERPClient{
		updateRemarksErr: assertErr("remarks update failed"),
	}
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleSupplier,
		DisplayName: "Abdulloh",
		Ref:         "SUP-001",
	})
	if err != nil {
		t.Fatalf("failed to create supplier session: %v", err)
	}

	commentReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/notifications/comments?receipt_id=MAT-PRE-0001",
		bytes.NewReader([]byte(`{"message":"Tasdiqlayman, shu holat bo'lganini ko'rdim."}`)),
	)
	commentReq.Header.Set("Authorization", "Bearer "+token)
	commentReq.Header.Set("Content-Type", "application/json")
	commentResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(commentResp, commentReq)
	if commentResp.Code != http.StatusOK {
		t.Fatalf("unexpected supplier acknowledgment status: %d body=%s", commentResp.Code, commentResp.Body.String())
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestServerAdminActivity(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleAdmin, DisplayName: "Admin"})
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/activity", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 1 || items[0].SupplierName != "Abdulloh" {
		t.Fatalf("unexpected activity payload: %+v", items)
	}
}

func TestServerAdminWerkaCodeRegenerate(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleAdmin, DisplayName: "Admin"})
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/mobile/admin/werka/code/regenerate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var settings AdminSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.HasPrefix(settings.WerkaCode, "20") {
		t.Fatalf("unexpected werka code: %q", settings.WerkaCode)
	}
}

func TestServerSupplierHistoryDeduplicatesReceipts(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			supplierReceipts: []erpnext.PurchaseReceiptDraft{
				{
					Name:                 "MAT-PRE-0002",
					Supplier:             "SUP-001",
					SupplierName:         "Abdulloh",
					SupplierDeliveryNote: "TG:+998900000000|64",
					ItemCode:             "ITEM-001",
					ItemName:             "Chers001",
					Qty:                  64,
					UOM:                  "Nos",
					PostingDate:          "2026-03-13",
				},
				{
					Name:                 "MAT-PRE-0002",
					Supplier:             "SUP-001",
					SupplierName:         "Abdulloh",
					SupplierDeliveryNote: "TG:+998900000000|64",
					ItemCode:             "ITEM-001",
					ItemName:             "Chers001",
					Qty:                  64,
					UOM:                  "Nos",
					PostingDate:          "2026-03-13",
				},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleSupplier,
		DisplayName: "Abdulloh",
		Ref:         "SUP-001",
	})
	if err != nil {
		t.Fatalf("failed to create supplier session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/supplier/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 deduped receipt, got %d", len(items))
	}
}

func TestServerWerkaHistoryDeduplicatesReceipts(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			telegramReceipts: []erpnext.PurchaseReceiptDraft{
				{
					Name:                 "MAT-PRE-0003",
					Supplier:             "SUP-001",
					SupplierName:         "Abdulloh",
					SupplierDeliveryNote: "TG:+998900000000|64",
					ItemCode:             "ITEM-001",
					ItemName:             "Chers001",
					Qty:                  64,
					UOM:                  "Nos",
					PostingDate:          "2026-03-13",
				},
				{
					Name:                 "MAT-PRE-0003",
					Supplier:             "SUP-001",
					SupplierName:         "Abdulloh",
					SupplierDeliveryNote: "TG:+998900000000|64",
					ItemCode:             "ITEM-001",
					ItemName:             "Chers001",
					Qty:                  64,
					UOM:                  "Nos",
					PostingDate:          "2026-03-13",
				},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleWerka,
		DisplayName: "Werka",
		Ref:         "werka",
	})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 deduped receipt, got %d", len(items))
	}
}

func TestServerWerkaHistoryReturnsCanonicalFullList(t *testing.T) {
	receipts := make([]erpnext.PurchaseReceiptDraft, 0, 80)
	for index := 0; index < 80; index++ {
		receipts = append(receipts, erpnext.PurchaseReceiptDraft{
			Name:                 fmt.Sprintf("MAT-PRE-%04d", index+1),
			Supplier:             "SUP-001",
			SupplierName:         "Abdulloh",
			SupplierDeliveryNote: fmt.Sprintf("TG:+998900000000|%d", index+1),
			ItemCode:             fmt.Sprintf("ITEM-%03d", index+1),
			ItemName:             fmt.Sprintf("Item %03d", index+1),
			Qty:                  1,
			UOM:                  "Kg",
			PostingDate:          "2026-03-20",
		})
	}
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{telegramReceipts: receipts},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleWerka,
		DisplayName: "Werka",
		Ref:         "werka",
	})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 80 {
		t.Fatalf("expected 80 Werka history items, got %d", len(items))
	}
}

func TestServerAdminActivityStillRespectsLimit(t *testing.T) {
	receipts := make([]erpnext.PurchaseReceiptDraft, 0, 40)
	for index := 0; index < 40; index++ {
		receipts = append(receipts, erpnext.PurchaseReceiptDraft{
			Name:                 fmt.Sprintf("MAT-PRE-%04d", index+1),
			Supplier:             "SUP-001",
			SupplierName:         "Abdulloh",
			SupplierDeliveryNote: fmt.Sprintf("TG:+998900000000|%d", index+1),
			ItemCode:             fmt.Sprintf("ITEM-%03d", index+1),
			ItemName:             fmt.Sprintf("Item %03d", index+1),
			Qty:                  1,
			UOM:                  "Kg",
			PostingDate:          "2026-03-20",
		})
	}
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{telegramReceipts: receipts},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleAdmin, DisplayName: "Admin"})
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/activity", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}

	var items []DispatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 30 {
		t.Fatalf("expected admin activity to remain limited to 30, got %d", len(items))
	}
}

func TestServerCustomerSummaryAndHistory(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			comments: map[string][]erpnext.Comment{
				"MAT-DN-0002": {{
					ID:        "l2",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-14 10:04:00",
				}, {
					ID:        "c2",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "confirmed", ""),
					CreatedAt: "2026-03-14 10:05:00",
				}},
				"MAT-DN-0003": {{
					ID:        "l3",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-14 10:09:00",
				}, {
					ID:        "c3",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "rejected", "xato"),
					CreatedAt: "2026-03-14 10:10:00",
				}},
			},
			customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
				{
					Name:                "MAT-DN-0001",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-001",
					ItemName:            "Chers",
					Qty:                 12,
					UOM:                 "Nos",
					PostingDate:         "2026-03-14",
					Status:              "Submitted",
					DocStatus:           1,
					AccordFlowState:     "1",
					AccordCustomerState: "0",
				},
				{
					Name:                "MAT-DN-0002",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-002",
					ItemName:            "Test",
					Qty:                 5,
					UOM:                 "Kg",
					PostingDate:         "2026-03-14",
					Status:              "Submitted",
					DocStatus:           1,
					AccordFlowState:     "1",
					AccordCustomerState: "3",
				},
				{
					Name:                 "MAT-DN-0003",
					Customer:             "CUST-001",
					CustomerName:         "Comfi",
					ItemCode:             "ITEM-003",
					ItemName:             "Reject",
					Qty:                  2,
					UOM:                  "Nos",
					PostingDate:          "2026-03-14",
					Status:               "Submitted",
					DocStatus:            1,
					AccordFlowState:      "1",
					AccordCustomerState:  "2",
					AccordCustomerReason: "xato",
				},
				{
					Name:         "MAT-DN-0004",
					Customer:     "CUST-001",
					CustomerName: "Comfi",
					ItemCode:     "ITEM-004",
					ItemName:     "Hidden draft",
					Qty:          1,
					UOM:          "Nos",
					PostingDate:  "2026-03-14",
					Status:       "Draft",
					DocStatus:    0,
				},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	token, err := server.sessions.Create(Principal{
		Role:        RoleCustomer,
		DisplayName: "Comfi",
		Ref:         "CUST-001",
		Phone:       "+998901000333",
	})
	if err != nil {
		t.Fatalf("failed to create customer session: %v", err)
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/customer/summary", nil)
	summaryReq.Header.Set("Authorization", "Bearer "+token)
	summaryResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(summaryResp, summaryReq)
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("unexpected summary status: %d body=%s", summaryResp.Code, summaryResp.Body.String())
	}

	var summary CustomerHomeSummary
	if err := json.NewDecoder(summaryResp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary failed: %v", err)
	}
	if summary.PendingCount != 1 || summary.ConfirmedCount != 1 || summary.RejectedCount != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/customer/history", nil)
	historyReq.Header.Set("Authorization", "Bearer "+token)
	historyResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(historyResp, historyReq)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("unexpected history status: %d body=%s", historyResp.Code, historyResp.Body.String())
	}

	var records []DispatchRecord
	if err := json.NewDecoder(historyResp.Body).Decode(&records); err != nil {
		t.Fatalf("decode history failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/customer/status-details?kind=pending", nil)
	detailReq.Header.Set("Authorization", "Bearer "+token)
	detailResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected details status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}

	var detailRecords []DispatchRecord
	if err := json.NewDecoder(detailResp.Body).Decode(&detailRecords); err != nil {
		t.Fatalf("decode details failed: %v", err)
	}
	if len(detailRecords) != 1 {
		t.Fatalf("expected 1 detail record, got %d", len(detailRecords))
	}
}

func TestServerCustomerDetailAndRespond(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			comments: map[string][]erpnext.Comment{
				"MAT-DN-0009": {{
					ID:        "l1",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-14 09:59:00",
				}, {
					ID:        "c1",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "pending", ""),
					CreatedAt: "2026-03-14 10:00:00",
				}},
			},
			customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
				{
					Name:                "MAT-DN-0009",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-001",
					ItemName:            "Chers",
					Qty:                 7,
					UOM:                 "Nos",
					PostingDate:         "2026-03-14",
					Status:              "Submitted",
					DocStatus:           1,
					AccordFlowState:     "1",
					AccordCustomerState: "0",
				},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))
	recorder := &recordingPushSender{}
	server.sender = recorder
	token, err := server.sessions.Create(Principal{
		Role:        RoleCustomer,
		DisplayName: "Comfi",
		Ref:         "CUST-001",
		Phone:       "+998901000333",
	})
	if err != nil {
		t.Fatalf("failed to create customer session: %v", err)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/customer/detail?delivery_note_id=MAT-DN-0009", nil)
	detailReq.Header.Set("Authorization", "Bearer "+token)
	detailResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected detail status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}

	var detail CustomerDeliveryDetail
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail failed: %v", err)
	}
	if !detail.CanApprove || !detail.CanReject {
		t.Fatalf("expected pending customer actions, got %+v", detail)
	}

	respondReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/customer/respond",
		strings.NewReader(`{"delivery_note_id":"MAT-DN-0009","approve":true}`),
	)
	respondReq.Header.Set("Authorization", "Bearer "+token)
	respondReq.Header.Set("Content-Type", "application/json")
	respondResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(respondResp, respondReq)
	if respondResp.Code != http.StatusOK {
		t.Fatalf("unexpected respond status: %d body=%s", respondResp.Code, respondResp.Body.String())
	}

	var updated CustomerDeliveryDetail
	if err := json.NewDecoder(respondResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode respond failed: %v", err)
	}
	if updated.Record.Status != "accepted" {
		t.Fatalf("expected accepted status, got %+v", updated.Record)
	}
	if updated.CanApprove || updated.CanReject {
		t.Fatalf("expected actions disabled after confirm, got %+v", updated)
	}
	if len(recorder.calls) != 2 {
		t.Fatalf("expected 2 push calls, got %d", len(recorder.calls))
	}
	if recorder.calls[0].key != "werka:werka" {
		t.Fatalf("unexpected first push key: %+v", recorder.calls[0])
	}
	if recorder.calls[0].data["event_type"] != "" {
		t.Fatalf("unexpected push event type before phase 5 feed shaping: %+v", recorder.calls[0].data)
	}
	if recorder.calls[1].key != "admin:admin" {
		t.Fatalf("unexpected second push key: %+v", recorder.calls[1])
	}
}

func TestServerWerkaAndAdminHistoryIncludeCustomerConfirmedResult(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			telegramReceipts: []erpnext.PurchaseReceiptDraft{},
			customers: []erpnext.Customer{
				{ID: "CUST-001", Name: "Comfi"},
			},
			customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
				{
					Name:                "MAT-DN-0011",
					Customer:            "CUST-001",
					CustomerName:        "Comfi",
					ItemCode:            "ITEM-001",
					ItemName:            "Chers",
					Qty:                 4,
					UOM:                 "Nos",
					PostingDate:         "2026-03-14",
					Status:              "Submitted",
					DocStatus:           1,
					AccordFlowState:     "1",
					AccordCustomerState: "3",
				},
				{
					Name:                 "MAT-DN-0012",
					Customer:             "CUST-001",
					CustomerName:         "Comfi",
					ItemCode:             "ITEM-002",
					ItemName:             "Reject",
					Qty:                  2,
					UOM:                  "Nos",
					PostingDate:          "2026-03-14",
					Status:               "Submitted",
					DocStatus:            1,
					AccordFlowState:      "1",
					AccordCustomerState:  "2",
					AccordCustomerReason: "Qabul qilinmadi",
				},
			},
			comments: map[string][]erpnext.Comment{
				"MAT-DN-0011": {{
					ID:        "LIFECYCLE-11",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-14 10:00:00",
				}, {
					ID:        "COMMENT-11",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "confirmed", ""),
					CreatedAt: "2026-03-14 10:10:00",
				}},
				"MAT-DN-0012": {{
					ID:        "LIFECYCLE-12",
					Content:   erpnext.BuildDeliveryLifecycleComment("submitted", "werka"),
					CreatedAt: "2026-03-14 10:01:00",
				}, {
					ID:        "COMMENT-12",
					Content:   erpnext.UpsertCustomerDecisionInRemarks("", "rejected", "Qabul qilinmadi"),
					CreatedAt: "2026-03-14 10:12:00",
				}},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
		nil,
	))

	werkaToken, err := server.sessions.Create(Principal{
		Role:        RoleWerka,
		DisplayName: "Werka",
		Ref:         "werka",
	})
	if err != nil {
		t.Fatalf("failed to create werka session: %v", err)
	}
	werkaReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/werka/history", nil)
	werkaReq.Header.Set("Authorization", "Bearer "+werkaToken)
	werkaResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(werkaResp, werkaReq)
	if werkaResp.Code != http.StatusOK {
		t.Fatalf("unexpected werka history status: %d body=%s", werkaResp.Code, werkaResp.Body.String())
	}
	var werkaRecords []DispatchRecord
	if err := json.NewDecoder(werkaResp.Body).Decode(&werkaRecords); err != nil {
		t.Fatalf("decode werka history failed: %v", err)
	}
	if len(werkaRecords) != 2 {
		t.Fatalf("expected 2 werka records, got %d", len(werkaRecords))
	}
	seenWerka := map[string]DispatchRecord{}
	for _, record := range werkaRecords {
		seenWerka[record.EventType] = record
	}
	if _, ok := seenWerka["customer_delivery_confirmed"]; !ok {
		t.Fatalf("missing confirmed werka event: %+v", werkaRecords)
	}
	rejectWerka, ok := seenWerka["customer_delivery_rejected"]
	if !ok {
		t.Fatalf("missing rejected werka event: %+v", werkaRecords)
	}
	if rejectWerka.Note != "Customer rad etdi. Sabab: Qabul qilinmadi" {
		t.Fatalf("unexpected reject note: %+v", rejectWerka)
	}

	adminToken, err := server.sessions.Create(Principal{
		Role:        RoleAdmin,
		DisplayName: "Admin",
		Ref:         "admin",
	})
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}
	adminReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/admin/activity", nil)
	adminReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin activity status: %d body=%s", adminResp.Code, adminResp.Body.String())
	}
	var adminRecords []DispatchRecord
	if err := json.NewDecoder(adminResp.Body).Decode(&adminRecords); err != nil {
		t.Fatalf("decode admin activity failed: %v", err)
	}
	if len(adminRecords) != 2 {
		t.Fatalf("expected 2 admin records, got %d", len(adminRecords))
	}
	seenAdmin := map[string]DispatchRecord{}
	for _, record := range adminRecords {
		seenAdmin[record.EventType] = record
	}
	if _, ok := seenAdmin["customer_delivery_confirmed"]; !ok {
		t.Fatalf("missing confirmed admin event: %+v", adminRecords)
	}
	if _, ok := seenAdmin["customer_delivery_rejected"]; !ok {
		t.Fatalf("missing rejected admin event: %+v", adminRecords)
	}
}

func TestServerCustomerRespondApproveReturnsAcceptedForSubmittedDeliveryNote(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
				{
					Name:         "MAT-DN-0010",
					Customer:     "CUST-001",
					CustomerName: "Comfi",
					ItemCode:     "pista93784",
					ItemName:     "pista",
					Qty:          12,
					UOM:          "Kg",
					PostingDate:  "2026-03-16",
					Status:       "Submitted",
					DocStatus:    1,
				},
			},
			comments: map[string][]erpnext.Comment{},
		},
		"http://erp.local",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"200000000000",
		"+998900000000",
		"Werka",
		NewProfileStore(t.TempDir()+"/profile_prefs.json"),
		NewAdminSupplierStore(t.TempDir()+"/admin_suppliers.json"),
	))
	server.sender = &recordingPushSender{}
	token, err := server.sessions.Create(Principal{
		Role:        RoleCustomer,
		DisplayName: "Comfi",
		Ref:         "CUST-001",
		Phone:       "+998901000333",
	})
	if err != nil {
		t.Fatalf("failed to create customer session: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/customer/respond",
		strings.NewReader(`{"delivery_note_id":"MAT-DN-0010","approve":true}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected respond status: %d body=%s", resp.Code, resp.Body.String())
	}

	var updated CustomerDeliveryDetail
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode respond failed: %v", err)
	}
	if updated.Record.Status != "accepted" {
		t.Fatalf("expected accepted status, got %+v", updated.Record)
	}
	if updated.CanApprove || updated.CanReject {
		t.Fatalf("expected actions disabled after confirm, got %+v", updated)
	}
}

func TestServerCustomerRespondPartialReturnsQtyAwareStatus(t *testing.T) {
	fake := &fakeERPClient{
		customerDeliveryNotes: []erpnext.DeliveryNoteDraft{
			{
				Name:                "MAT-DN-0013",
				Customer:            "CUST-001",
				CustomerName:        "Comfi",
				ItemCode:            "ITEM-013",
				ItemName:            "Pista",
				Qty:                 10,
				UOM:                 "Kg",
				PostingDate:         "2026-03-16",
				Status:              "Submitted",
				DocStatus:           1,
				AccordFlowState:     "1",
				AccordCustomerState: "1",
			},
		},
		comments: map[string][]erpnext.Comment{},
	}
	server := NewServer(NewERPAuthenticator(
		fake,
		"http://erp.local",
		"key",
		"secret",
		"Stores - A",
		"10",
		"20",
		"200000000000",
		"+998900000000",
		"Werka",
		NewProfileStore(t.TempDir()+"/profile_prefs.json"),
		NewAdminSupplierStore(t.TempDir()+"/admin_suppliers.json"),
	))
	server.sender = &recordingPushSender{}
	token, err := server.sessions.Create(Principal{
		Role:        RoleCustomer,
		DisplayName: "Comfi",
		Ref:         "CUST-001",
		Phone:       "+998901000333",
	})
	if err != nil {
		t.Fatalf("failed to create customer session: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/mobile/customer/respond",
		strings.NewReader(`{"delivery_note_id":"MAT-DN-0013","mode":"accept_partial","accepted_qty":7,"returned_qty":3,"reason":"Brak chiqdi"}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected respond status: %d body=%s", resp.Code, resp.Body.String())
	}

	var updated CustomerDeliveryDetail
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode respond failed: %v", err)
	}
	if updated.Record.Status != "partial" {
		t.Fatalf("expected partial status, got %+v", updated.Record)
	}
	if updated.Record.AcceptedQty != 7 {
		t.Fatalf("expected accepted qty 7, got %+v", updated.Record)
	}
	if fake.lastDeliveryReturn != "MAT-DN-0013" || fake.lastDeliveryReturnQty != 3 {
		t.Fatalf("expected qty-aware return against source, got source=%q qty=%.2f", fake.lastDeliveryReturn, fake.lastDeliveryReturnQty)
	}
}

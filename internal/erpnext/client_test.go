package erpnext

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateCredentialsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "token key:secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			_, _ = w.Write([]byte(`{"message":["Stock User","Material Manager"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "user@example.com" {
		t.Fatalf("unexpected username: %q", result.Username)
	}
	if len(result.Roles) != 2 {
		t.Fatalf("expected 2 roles, got: %v", result.Roles)
	}
}

func TestCredentialProviderOverridesCallSiteCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "token liveKey:liveSecret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			_, _ = w.Write([]byte(`{"message":["Stock User"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	client.SetCredentialProvider(func(context.Context) (string, string, error) {
		return "liveKey", "liveSecret", nil
	})

	result, err := client.ValidateCredentials(context.Background(), server.URL, "staleKey", "staleSecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "user@example.com" {
		t.Fatalf("unexpected username: %q", result.Username)
	}
}

func TestValidateCredentialsFallbackRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"user@example.com"}`))
		case "/api/method/frappe.core.doctype.user.user.get_roles":
			http.NotFound(w, r)
		case "/api/resource/User/user@example.com":
			_, _ = w.Write([]byte(`{"data":{"roles":[{"role":"Stock User"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.ValidateCredentials(context.Background(), server.URL, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "Stock User" {
		t.Fatalf("unexpected roles: %v", result.Roles)
	}
}

func TestValidateCredentialsInvalidAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.ValidateCredentials(context.Background(), server.URL, "bad", "bad")
	if err == nil {
		t.Fatal("expected error for invalid auth")
	}
}

func TestApplyPartialDeliveryReturnQtyUpdatesFirstItem(t *testing.T) {
	doc := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"item_code": "ITEM-001",
				"qty":       -10.0,
			},
		},
	}

	if err := applyPartialDeliveryReturnQty(doc, 3); err != nil {
		t.Fatalf("applyPartialDeliveryReturnQty() error = %v", err)
	}

	items := doc["items"].([]interface{})
	first := items[0].(map[string]interface{})
	if got := first["qty"]; got != -3.0 {
		t.Fatalf("expected qty -3.0, got %+v", got)
	}
}

func TestCustomerDecisionRemarksRoundTrip(t *testing.T) {
	remarks := UpsertCustomerDecisionPayloadInRemarks(
		"",
		"partial",
		"Brak chiqdi",
		7,
		3,
		"Kg",
		"3 kg qaytdi",
	)

	if got := ExtractCustomerDecisionState(remarks); got != "partial" {
		t.Fatalf("unexpected state: %q", got)
	}
	if got := ExtractCustomerDecisionReason(remarks); got != "Brak chiqdi" {
		t.Fatalf("unexpected reason: %q", got)
	}
	if got := ExtractCustomerDecisionComment(remarks); got != "3 kg qaytdi" {
		t.Fatalf("unexpected comment: %q", got)
	}
	acceptedQty, returnedQty := ExtractCustomerDecisionQuantities(remarks)
	if acceptedQty != 7 || returnedQty != 3 {
		t.Fatalf("unexpected quantities: accepted=%.2f returned=%.2f", acceptedQty, returnedQty)
	}
}

func TestSearchItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Item" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Rice","stock_uom":"Kg"}]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.SearchItems(context.Background(), server.URL, "key", "secret", "ri", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Code != "ITEM-001" || items[0].UOM != "Kg" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

func TestSearchItemsHonorsHigherLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Item" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("limit_page_length"); got != "200" {
			t.Fatalf("expected limit_page_length=200, got %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.SearchItems(context.Background(), server.URL, "key", "secret", "", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchSupplierItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			if r.URL.Query().Get("doctype") != "Supplier" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"message":[{"value":"SUP-001"}]}`))
		case "/api/resource/Item Supplier":
			if got := r.URL.Query().Get("parent"); got != "Item" {
				http.Error(w, "missing parent doctype", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"parent":"ITEM-001"}]}`))
		case "/api/resource/Item":
			_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Rice","stock_uom":"Kg"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.SearchSupplierItems(context.Background(), server.URL, "key", "secret", "Abdulloh", "ri", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Code != "ITEM-001" || items[0].UOM != "Kg" {
		t.Fatalf("unexpected supplier items: %+v", items)
	}
}

func TestCreateItemUsesProvidedItemGroup(t *testing.T) {
	var body map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Item" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":{"name":"ITEM-NEW","item_name":"Test Item","stock_uom":"Kg"}}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	item, err := client.CreateItem(context.Background(), server.URL, "key", "secret", CreateItemInput{
		Code:      "ITEM-NEW",
		Name:      "Test Item",
		UOM:       "Kg",
		ItemGroup: "Tayyor mahsulot",
	})
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	if item.Code != "ITEM-NEW" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if got := body["item_group"]; got != "Tayyor mahsulot" {
		t.Fatalf("expected item_group Tayyor mahsulot, got %+v", got)
	}
}

func TestListPurchaseReceiptCommentsBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Comment" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[
			{"name":"COMM-001","content":"First","creation":"2026-03-10 10:00:00","reference_name":"MAT-PRE-0001"},
			{"name":"COMM-002","content":"Second","creation":"2026-03-10 10:05:00","reference_name":"MAT-PRE-0001"},
			{"name":"COMM-003","content":"Third","creation":"2026-03-10 11:00:00","reference_name":"MAT-PRE-0002"}
		]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	itemsByName, err := client.ListPurchaseReceiptCommentsBatch(
		context.Background(),
		server.URL,
		"key",
		"secret",
		[]string{"MAT-PRE-0001", "MAT-PRE-0002"},
		10,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(itemsByName["MAT-PRE-0001"]) != 2 {
		t.Fatalf("expected 2 comments for MAT-PRE-0001, got %+v", itemsByName["MAT-PRE-0001"])
	}
	if len(itemsByName["MAT-PRE-0002"]) != 1 {
		t.Fatalf("expected 1 comment for MAT-PRE-0002, got %+v", itemsByName["MAT-PRE-0002"])
	}
}

func TestSearchSuppliers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Supplier" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"SUP-001","supplier_name":"Abdulloh","mobile_no":"+998901234567","supplier_details":"Telefon: +998901234567"}]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.SearchSuppliers(context.Background(), server.URL, "key", "secret", "abd", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Abdulloh" || items[0].Phone != "+998901234567" {
		t.Fatalf("unexpected suppliers: %+v", items)
	}
}

func TestEnsureSupplierCreatesWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/resource/Supplier":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"data":{"name":"SUP-001","supplier_name":"Ali","mobile_no":"+998901234567"}}`))
				return
			}
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	supplier, err := client.EnsureSupplier(context.Background(), server.URL, "key", "secret", CreateSupplierInput{
		Name:  "Ali",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if supplier.ID != "SUP-001" || supplier.Name != "Ali" || supplier.Phone != "+998901234567" {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
}

func TestSearchSuppliersFallsBackToSupplierDetailsPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Supplier" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"SUP-001","supplier_name":"Abdulloh","mobile_no":"","supplier_details":"Telefon: +998901234567"}]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.SearchSuppliers(context.Background(), server.URL, "key", "secret", "abd", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Phone != "+998901234567" {
		t.Fatalf("unexpected suppliers: %+v", items)
	}
}

func TestGetSupplier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Supplier/SUP-001" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"name":"SUP-001","supplier_name":"Abdulloh","mobile_no":"+998901234567","image":"/files/avatar.png"}}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	item, err := client.GetSupplier(context.Background(), server.URL, "key", "secret", "SUP-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "SUP-001" || item.Image != "/files/avatar.png" || item.Phone != "+998901234567" {
		t.Fatalf("unexpected supplier: %+v", item)
	}
}

func TestUploadSupplierImage(t *testing.T) {
	uploaded := false
	updated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/upload_file":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			uploaded = true
			_, _ = w.Write([]byte(`{"message":{"file_url":"/files/avatar.png"}}`))
		case "/api/resource/Supplier/SUP-001":
			if r.Method != http.MethodPut {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			updated = true
			_, _ = w.Write([]byte(`{"data":{"name":"SUP-001"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	fileURL, err := client.UploadSupplierImage(context.Background(), server.URL, "key", "secret", "SUP-001", "avatar.png", "image/png", []byte("pngdata"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fileURL != "/files/avatar.png" || !uploaded || !updated {
		t.Fatalf("unexpected upload result: %q uploaded=%v updated=%v", fileURL, uploaded, updated)
	}
}

func TestCreateAndSubmitStockEntry(t *testing.T) {
	var createPayload map[string]interface{}
	var submitPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/resource/Stock Entry/") || strings.HasPrefix(r.URL.EscapedPath(), "/api/resource/Stock%20Entry/") {
			if r.Method != http.MethodGet {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"doctype":"Stock Entry","name":"STE-0001","modified":"2026-03-05 10:00:00.000000"}}`))
			return
		}

		switch r.URL.Path {
		case "/api/resource/Stock Entry":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &createPayload)
			_, _ = w.Write([]byte(`{"data":{"name":"STE-0001"}}`))
		case "/api/method/frappe.client.submit":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &submitPayload)
			_, _ = w.Write([]byte(`{"message":{"name":"STE-0001","docstatus":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	result, err := client.CreateAndSubmitStockEntry(
		context.Background(),
		server.URL,
		"key",
		"secret",
		CreateStockEntryInput{
			EntryType:       "Material Receipt",
			ItemCode:        "ITEM-001",
			Qty:             5,
			UOM:             "Kg",
			TargetWarehouse: "Stores - CH",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "STE-0001" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if createPayload["stock_entry_type"] != "Material Receipt" {
		t.Fatalf("unexpected create payload: %+v", createPayload)
	}
	if submitPayload["doc"] == nil {
		t.Fatalf("unexpected submit payload: %+v", submitPayload)
	}
}

func TestCreateDraftAndSubmitPurchaseReceipt(t *testing.T) {
	var updatePayload map[string]interface{}
	var submitPayload map[string]interface{}

	docResponse := `{"data":{"doctype":"Purchase Receipt","name":"MAT-PRE-0001","supplier":"SUP-001","posting_date":"2026-03-07","supplier_delivery_note":"TG:+998901234567:20260307120000","items":[{"item_code":"ITEM-001","item_name":"Rice","qty":10,"uom":"Kg","stock_uom":"Kg","warehouse":"Stores - CH","conversion_factor":1}]}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			_, _ = w.Write([]byte(`{"message":[{"value":"SUP-001"}]}`))
		case "/api/resource/Warehouse/Stores - CH", "/api/resource/Warehouse/Stores%20-%20CH":
			_, _ = w.Write([]byte(`{"data":{"company":"_Test Company"}}`))
		case "/api/resource/Purchase Receipt":
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"name":"MAT-PRE-0001"}}`))
		case "/api/resource/Purchase Receipt/MAT-PRE-0001", "/api/resource/Purchase%20Receipt/MAT-PRE-0001":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(docResponse))
			case http.MethodPut:
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &updatePayload)
				_, _ = w.Write([]byte(`{"data":{"name":"MAT-PRE-0001"}}`))
			default:
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
			}
		case "/api/method/frappe.client.submit":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &submitPayload)
			_, _ = w.Write([]byte(`{"message":{"name":"MAT-PRE-0001","docstatus":1}}`))
		case "/api/resource/Item":
			_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Rice","stock_uom":"Kg"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	draft, err := client.CreateDraftPurchaseReceipt(context.Background(), server.URL, "key", "secret", CreatePurchaseReceiptInput{
		Supplier:      "Abdulloh",
		SupplierPhone: "+998901234567",
		ItemCode:      "ITEM-001",
		Qty:           10,
		UOM:           "Kg",
		Warehouse:     "Stores - CH",
	})
	if err != nil {
		t.Fatalf("unexpected draft create error: %v", err)
	}
	if draft.Name != "MAT-PRE-0001" || draft.ItemCode != "ITEM-001" {
		t.Fatalf("unexpected draft: %+v", draft)
	}

	result, err := client.ConfirmAndSubmitPurchaseReceipt(context.Background(), server.URL, "key", "secret", "MAT-PRE-0001", 7, 0, "", "")
	if err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}
	if result.Name != "MAT-PRE-0001" || result.AcceptedQty != 7 || result.SentQty != 10 {
		t.Fatalf("unexpected submit result: %+v", result)
	}

	items, ok := updatePayload["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected update payload: %+v", updatePayload)
	}
	first, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected item payload: %+v", items[0])
	}
	if first["qty"] != float64(7) || first["received_qty"] != float64(7) {
		t.Fatalf("unexpected updated item payload: %+v", first)
	}
	if submitPayload["doc"] == nil {
		t.Fatalf("unexpected submit payload: %+v", submitPayload)
	}
}

func TestEnsureDeliveryNoteStateFieldsCachesSuccessfulCheck(t *testing.T) {
	var getCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/resource/Custom Field":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method for custom field check: %s", r.Method)
			}
			atomic.AddInt32(&getCalls, 1)
			_, _ = w.Write([]byte(`{"data":[
				{"name":"CF-001","fieldname":"accord_flow_state","label":"Accord Flow State","fieldtype":"Int","insert_after":"remarks","hidden":1,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":""},
				{"name":"CF-002","fieldname":"accord_customer_state","label":"Accord Customer State","fieldtype":"Int","insert_after":"accord_flow_state","hidden":1,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":""},
				{"name":"CF-003","fieldname":"accord_customer_reason","label":"Accord Customer Reason","fieldtype":"Small Text","insert_after":"accord_customer_state","hidden":1,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":""},
				{"name":"CF-004","fieldname":"accord_delivery_actor","label":"Accord Delivery Actor","fieldtype":"Data","insert_after":"accord_customer_reason","hidden":1,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":""},
				{"name":"CF-005","fieldname":"accord_status_section","label":"Accord Status","fieldtype":"Section Break","insert_after":"posting_time","hidden":0,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":""},
				{"name":"CF-006","fieldname":"accord_ui_status","label":"Accord UI Status","fieldtype":"Select","insert_after":"accord_status_section","hidden":0,"read_only":1,"allow_on_submit":1,"no_copy":1,"options":"pending\nconfirm\npartial\nrejected"}
			]}`))
		case r.URL.Path == "/api/resource/Custom Field/CF-001",
			r.URL.Path == "/api/resource/Custom Field/CF-002",
			r.URL.Path == "/api/resource/Custom Field/CF-003",
			r.URL.Path == "/api/resource/Custom Field/CF-004",
			r.URL.Path == "/api/resource/Custom Field/CF-005",
			r.URL.Path == "/api/resource/Custom Field/CF-006":
			t.Fatalf("did not expect custom field write request: %s %s", r.Method, r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	if err := client.EnsureDeliveryNoteStateFields(context.Background(), server.URL, "key", "secret"); err != nil {
		t.Fatalf("first EnsureDeliveryNoteStateFields() error = %v", err)
	}
	if err := client.EnsureDeliveryNoteStateFields(context.Background(), server.URL, "key", "secret"); err != nil {
		t.Fatalf("second EnsureDeliveryNoteStateFields() error = %v", err)
	}
	if got := atomic.LoadInt32(&getCalls); got != 1 {
		t.Fatalf("expected exactly 1 custom field lookup, got %d", got)
	}
}

func TestConfirmPurchaseReceiptClearsRejectedWarehouseWhenSameAsAccepted(t *testing.T) {
	var updatePayload map[string]interface{}

	docResponse := `{"data":{"doctype":"Purchase Receipt","name":"MAT-PRE-0002","supplier":"SUP-001","posting_date":"2026-03-07","supplier_delivery_note":"TG:+998901234567:20260307120000","items":[{"item_code":"ITEM-001","item_name":"Rice","qty":10,"uom":"Kg","stock_uom":"Kg","warehouse":"Stores - CH","rejected_warehouse":"Stores - CH","conversion_factor":1}]}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/resource/Warehouse":
			_, _ = w.Write([]byte(`{"data":[{"name":"All Warehouses - A","is_group":1},{"name":"Stores - CH","is_group":0},{"name":"Finished Goods - A","is_group":0}]}`))
		case "/api/resource/Purchase Receipt/MAT-PRE-0002", "/api/resource/Purchase%20Receipt/MAT-PRE-0002":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(docResponse))
			case http.MethodPut:
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &updatePayload)
				_, _ = w.Write([]byte(`{"data":{"name":"MAT-PRE-0002"}}`))
			default:
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
			}
		case "/api/method/frappe.client.submit":
			_, _ = w.Write([]byte(`{"message":{"name":"MAT-PRE-0002","docstatus":1}}`))
		case "/api/resource/Comment":
			_, _ = w.Write([]byte(`{"data":{"name":"COMM-0001"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.ConfirmAndSubmitPurchaseReceipt(context.Background(), server.URL, "key", "secret", "MAT-PRE-0002", 7, 3, "Yaroqsiz", "partial test")
	if err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}

	items, ok := updatePayload["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected update payload: %+v", updatePayload)
	}
	first, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected item payload: %+v", items[0])
	}
	if got := first["rejected_warehouse"]; got != "Finished Goods - A" {
		t.Fatalf("expected alternate rejected_warehouse, got %+v", got)
	}
	if got := first["received_qty"]; got != float64(10) {
		t.Fatalf("expected received_qty to include accepted+rejected, got %+v", got)
	}
}

func TestConfirmPurchaseReceiptRollsBackDraftWhenSubmitFails(t *testing.T) {
	updateBodies := make([]map[string]interface{}, 0, 2)

	docResponse := `{"data":{"doctype":"Purchase Receipt","name":"MAT-PRE-0003","supplier":"SUP-001","posting_date":"2026-03-07","supplier_delivery_note":"TG:+998901234567:20260307120000:10.0000","remarks":"","items":[{"item_code":"ITEM-001","item_name":"Rice","qty":10,"received_qty":10,"uom":"Kg","stock_uom":"Kg","warehouse":"Stores - CH","conversion_factor":1}]}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/resource/Warehouse":
			_, _ = w.Write([]byte(`{"data":[{"name":"Stores - CH","is_group":0},{"name":"Finished Goods - A","is_group":0}]}`))
		case "/api/resource/Purchase Receipt/MAT-PRE-0003", "/api/resource/Purchase%20Receipt/MAT-PRE-0003":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(docResponse))
			case http.MethodPut:
				raw, _ := io.ReadAll(r.Body)
				var body map[string]interface{}
				_ = json.Unmarshal(raw, &body)
				updateBodies = append(updateBodies, body)
				_, _ = w.Write([]byte(`{"data":{"name":"MAT-PRE-0003"}}`))
			default:
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
			}
		case "/api/method/frappe.client.submit":
			http.Error(w, `{"exception":"submit failed"}`, http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	_, err := client.ConfirmAndSubmitPurchaseReceipt(context.Background(), server.URL, "key", "secret", "MAT-PRE-0003", 7, 3, "Yaroqsiz", "rollback test")
	if err == nil {
		t.Fatal("expected submit error")
	}
	if len(updateBodies) != 2 {
		t.Fatalf("expected 2 updates (apply + rollback), got %d", len(updateBodies))
	}
	secondItems, ok := updateBodies[1]["items"].([]interface{})
	if !ok || len(secondItems) != 1 {
		t.Fatalf("unexpected rollback payload: %+v", updateBodies[1])
	}
	second, ok := secondItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected rollback item payload: %+v", secondItems[0])
	}
	if second["qty"] != float64(10) {
		t.Fatalf("expected rollback to restore qty=10, got %+v", second["qty"])
	}
}

func TestSearchWarehousesAndUOMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			doctype := r.URL.Query().Get("doctype")
			if doctype == "Warehouse" {
				_, _ = w.Write([]byte(`{"message":[{"value":"Stores - CH"}]}`))
				return
			}
			if doctype == "UOM" {
				_, _ = w.Write([]byte(`{"message":[{"value":"Kg"},{"value":"Nos"}]}`))
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})

	warehouses, err := client.SearchWarehouses(context.Background(), server.URL, "key", "secret", "store", 10)
	if err != nil {
		t.Fatalf("unexpected warehouse error: %v", err)
	}
	if len(warehouses) != 1 || warehouses[0].Name != "Stores - CH" {
		t.Fatalf("unexpected warehouses: %+v", warehouses)
	}

	uoms, err := client.SearchUOMs(context.Background(), server.URL, "key", "secret", "k", 10)
	if err != nil {
		t.Fatalf("unexpected uom error: %v", err)
	}
	if len(uoms) != 2 || uoms[0].Name == "" {
		t.Fatalf("unexpected uoms: %+v", uoms)
	}
}

func TestListPendingPurchaseReceiptsReturnsAllDrafts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/resource/Purchase Receipt":
			_, _ = w.Write([]byte(`{"data":[{"name":"MAT-PRE-0001"},{"name":"MAT-PRE-0002"}]}`))
		case r.URL.Path == "/api/resource/Purchase%20Receipt/MAT-PRE-0001" || r.URL.EscapedPath() == "/api/resource/Purchase%20Receipt/MAT-PRE-0001":
			_, _ = w.Write([]byte(`{"data":{"doctype":"Purchase Receipt","name":"MAT-PRE-0001","supplier":"SUP-001","posting_date":"2026-03-09","supplier_delivery_note":"","items":[{"item_code":"ITEM-001","item_name":"Rice","qty":10,"uom":"Kg","stock_uom":"Kg","warehouse":"Stores - A","conversion_factor":1}]}}`))
		case r.URL.Path == "/api/resource/Purchase%20Receipt/MAT-PRE-0002" || r.URL.EscapedPath() == "/api/resource/Purchase%20Receipt/MAT-PRE-0002":
			_, _ = w.Write([]byte(`{"data":{"doctype":"Purchase Receipt","name":"MAT-PRE-0002","supplier":"SUP-002","posting_date":"2026-03-09","supplier_delivery_note":"TG:+998901234567:20260309120000","items":[{"item_code":"ITEM-002","item_name":"Oil","qty":5,"uom":"L","stock_uom":"L","warehouse":"Stores - A","conversion_factor":1}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.ListPendingPurchaseReceipts(context.Background(), server.URL, "key", "secret", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 pending drafts, got %d", len(items))
	}
	if items[0].Name != "MAT-PRE-0001" || items[1].Name != "MAT-PRE-0002" {
		t.Fatalf("unexpected drafts: %+v", items)
	}
}

func TestListPendingPurchaseReceiptsUsesInlineDataWhenAvailable(t *testing.T) {
	detailFetches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/resource/Purchase Receipt":
			_, _ = w.Write([]byte(`{"data":[{"name":"MAT-PRE-0001","supplier":"SUP-001","supplier_name":"Abdulloh","posting_date":"2026-03-09","supplier_delivery_note":"","status":"Draft","docstatus":0,"currency":"UZS","remarks":"","items":[{"item_code":"ITEM-001","item_name":"Rice","qty":10,"amount":120,"uom":"Kg","stock_uom":"Kg","warehouse":"Stores - A","conversion_factor":1}]}]}`))
		case strings.Contains(r.URL.Path, "/api/resource/Purchase%20Receipt/"):
			detailFetches++
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	items, err := client.ListPendingPurchaseReceipts(context.Background(), server.URL, "key", "secret", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "MAT-PRE-0001" || items[0].Amount != 120 {
		t.Fatalf("unexpected drafts: %+v", items)
	}
	if detailFetches != 0 {
		t.Fatalf("expected no detail fetches, got %d", detailFetches)
	}
}

func TestBuildSearchQueryVariantsAddsLatinFallbackForCyrillic(t *testing.T) {
	variants := buildSearchQueryVariants("омбор")
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %v", variants)
	}
	if variants[0] != "омбор" {
		t.Fatalf("unexpected first variant: %q", variants[0])
	}
	if variants[1] != "ombor" {
		t.Fatalf("unexpected latin fallback: %q", variants[1])
	}
}

func TestSearchWarehousesUsesCyrillicFallbackVariant(t *testing.T) {
	var queries []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/method/frappe.desk.search.search_link" {
			http.NotFound(w, r)
			return
		}

		query, err := url.QueryUnescape(r.URL.Query().Get("txt"))
		if err != nil {
			t.Fatalf("failed to decode query: %v", err)
		}
		queries = append(queries, query)

		if query == "ombor" {
			_, _ = w.Write([]byte(`{"message":[{"value":"Stores - CH"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":[]}`))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: 3 * time.Second})
	warehouses, err := client.SearchWarehouses(context.Background(), server.URL, "key", "secret", "омбор", 10)
	if err != nil {
		t.Fatalf("unexpected warehouse error: %v", err)
	}
	if len(warehouses) != 1 || warehouses[0].Name != "Stores - CH" {
		t.Fatalf("unexpected warehouses: %+v", warehouses)
	}
	if len(queries) < 2 {
		t.Fatalf("expected fallback queries, got %v", queries)
	}
	if queries[0] != "омбор" || queries[1] != "ombor" {
		t.Fatalf("unexpected queries order: %v", queries)
	}
}

func TestIsDuplicateBarcodeError(t *testing.T) {
	t.Run("matches duplicate barcode message", func(t *testing.T) {
		err := errors.New(`status 417: {"exception":"IntegrityError(1062, \"Duplicate entry '4780092350042' for key 'barcode'\")"}`)
		if !isDuplicateBarcodeError(err) {
			t.Fatal("expected duplicate barcode error to match")
		}
	})

	t.Run("ignores unrelated error", func(t *testing.T) {
		err := errors.New("status 500: boom")
		if isDuplicateBarcodeError(err) {
			t.Fatal("expected unrelated error not to match")
		}
	})
}

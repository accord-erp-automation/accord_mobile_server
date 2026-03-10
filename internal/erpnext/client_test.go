package erpnext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestSearchSupplierItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.desk.search.search_link":
			if r.URL.Query().Get("doctype") != "Supplier" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"message":[{"value":"SUP-001"}]}`))
		case "/api/resource/Item":
			_, _ = w.Write([]byte(`{"data":[{"name":"ITEM-001","item_name":"Rice","stock_uom":"Kg"}]}`))
		case "/api/resource/Item/ITEM-001":
			_, _ = w.Write([]byte(`{"data":{"default_supplier":"","supplier_items":[{"supplier":"SUP-001"}]}}`))
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

	result, err := client.ConfirmAndSubmitPurchaseReceipt(context.Background(), server.URL, "key", "secret", "MAT-PRE-0001", 7)
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

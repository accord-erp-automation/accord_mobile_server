package importacp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mobile_server/internal/erpnext"
)

type stubERP struct {
	customers        []erpnext.Customer
	itemsByCode      map[string]erpnext.Item
	assignByCode     map[string]erpnext.ItemCustomerAssignment
	createdCustomers []erpnext.CreateCustomerInput
	createdItems     []erpnext.CreateItemInput
	assignments      [][2]string
	priceUpdates     map[string]float64
	barcodeUpdates   map[string]string
}

func (s *stubERP) SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error) {
	return s.customers, nil
}

func (s *stubERP) EnsureCustomer(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateCustomerInput) (erpnext.Customer, error) {
	s.createdCustomers = append(s.createdCustomers, input)
	customer := erpnext.Customer{ID: input.Name, Name: input.Name}
	s.customers = append(s.customers, customer)
	return customer, nil
}

func (s *stubERP) GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error) {
	result := make([]erpnext.Item, 0, len(itemCodes))
	for _, code := range itemCodes {
		if item, ok := s.itemsByCode[strings.TrimSpace(code)]; ok {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *stubERP) GetItemCustomerAssignment(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (erpnext.ItemCustomerAssignment, error) {
	if item, ok := s.assignByCode[strings.TrimSpace(itemCode)]; ok {
		return item, nil
	}
	return erpnext.ItemCustomerAssignment{Code: strings.TrimSpace(itemCode)}, nil
}

func (s *stubERP) CreateItem(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateItemInput) (erpnext.Item, error) {
	s.createdItems = append(s.createdItems, input)
	item := erpnext.Item{Code: input.Code, Name: input.Name, UOM: input.UOM}
	if s.itemsByCode == nil {
		s.itemsByCode = map[string]erpnext.Item{}
	}
	s.itemsByCode[item.Code] = item
	return item, nil
}

func (s *stubERP) AssignCustomerToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error {
	s.assignments = append(s.assignments, [2]string{itemCode, customerRef})
	return nil
}

func (s *stubERP) UpdateItemStandardRate(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string, rate float64) error {
	if s.priceUpdates == nil {
		s.priceUpdates = map[string]float64{}
	}
	s.priceUpdates[itemCode] = rate
	return nil
}

func (s *stubERP) UpsertItemBarcode(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, barcode, uom string) error {
	if strings.TrimSpace(barcode) == "" {
		return nil
	}
	if s.barcodeUpdates == nil {
		s.barcodeUpdates = map[string]string{}
	}
	s.barcodeUpdates[itemCode] = barcode
	return nil
}

func TestRunCreatesCustomersItemsAndPriceBarcode(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "acp.csv")
	content := "Agent,Nom,Price,Barcode\nMakiz,Test A,\"5,6\",12345\nMakiz,Test B,3,\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	stub := &stubERP{}
	result, err := Run(context.Background(), stub, nil, Options{
		CSVPath:   csvPath,
		UOM:       "Kg",
		ItemGroup: "Tayyor mahsulot",
		BaseURL:   "http://erp.test",
		APIKey:    "key",
		APISecret: "secret",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.CustomersCreated) != 1 || result.CustomersCreated[0] != "Makiz" {
		t.Fatalf("unexpected customers created: %+v", result.CustomersCreated)
	}
	if len(stub.createdItems) != 2 {
		t.Fatalf("expected 2 created items, got %+v", stub.createdItems)
	}
	if stub.createdItems[0].ItemGroup != "Tayyor mahsulot" {
		t.Fatalf("unexpected item group: %+v", stub.createdItems[0])
	}
	if stub.priceUpdates["Test A"] != 5.6 || stub.priceUpdates["Test B"] != 3 {
		t.Fatalf("unexpected price updates: %+v", stub.priceUpdates)
	}
	if stub.barcodeUpdates["Test A"] != "12345" {
		t.Fatalf("unexpected barcode updates: %+v", stub.barcodeUpdates)
	}
}

func TestRunBlocksConflictWithOtherCustomer(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "acp.csv")
	content := "Agent,Nom,Price,Barcode\nMakiz,Shared Item,2,\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	stub := &stubERP{
		customers: []erpnext.Customer{{ID: "Makiz", Name: "Makiz"}},
		itemsByCode: map[string]erpnext.Item{
			"Shared Item": {Code: "Shared Item", Name: "Shared Item", UOM: "Kg"},
		},
		assignByCode: map[string]erpnext.ItemCustomerAssignment{
			"Shared Item": {Code: "Shared Item", CustomerRefs: []string{"Other Customer"}},
		},
	}
	_, err := Run(context.Background(), stub, nil, Options{
		CSVPath:   csvPath,
		UOM:       "Kg",
		ItemGroup: "Tayyor mahsulot",
		BaseURL:   "http://erp.test",
		APIKey:    "key",
		APISecret: "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "already linked to customer") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

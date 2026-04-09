package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const telegramReceiptMarkerPrefix = "TG:"

type CreatePurchaseReceiptInput struct {
	Supplier      string
	SupplierPhone string
	ItemCode      string
	Qty           float64
	UOM           string
	Warehouse     string
}

type PurchaseReceiptDraft struct {
	Name                 string
	DocStatus            int
	Status               string
	Supplier             string
	SupplierName         string
	PostingDate          string
	SupplierDeliveryNote string
	ItemCode             string
	ItemName             string
	Qty                  float64
	UOM                  string
	Warehouse            string
	Amount               float64
	Currency             string
	Remarks              string
}

type PurchaseReceiptSubmissionResult struct {
	Name                 string
	Supplier             string
	ItemCode             string
	UOM                  string
	SentQty              float64
	AcceptedQty          float64
	SupplierDeliveryNote string
	Note                 string
}

type Comment struct {
	ID        string
	Content   string
	CreatedAt string
}

func (c *Client) SearchSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier, query string, limit int) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return nil, err
	}

	itemCodes, err := c.fetchSupplierItemCodes(ctx, normalized, apiKey, apiSecret, supplierLink, 500)
	if err != nil {
		return nil, err
	}
	return c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, itemCodes, query, limit)
}

func (c *Client) ListAssignedSupplierItems(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return nil, err
	}
	itemCodes, err := c.fetchSupplierItemCodes(ctx, normalized, apiKey, apiSecret, supplierLink, limit)
	if err != nil {
		return nil, err
	}
	return c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, itemCodes, "", limit)
}

func (c *Client) AssignSupplierToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return err
	}
	match, err := c.itemHasSupplier(ctx, normalized, apiKey, apiSecret, itemCode, supplierLink)
	if err != nil {
		return err
	}
	if match {
		return nil
	}
	endpoint := normalized + "/api/resource/Item%20Supplier"
	return c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, map[string]interface{}{
		"parent":      strings.TrimSpace(itemCode),
		"parenttype":  "Item",
		"parentfield": "supplier_items",
		"supplier":    supplierLink,
	}, nil)
}

func (c *Client) RemoveSupplierFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, supplier string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, supplier)
	if err != nil {
		return err
	}

	var payload struct {
		Data struct {
			DefaultSupplier string `json:"default_supplier"`
			SupplierItems   []struct {
				Name     string `json:"name"`
				Supplier string `json:"supplier"`
			} `json:"supplier_items"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return err
	}

	for _, row := range payload.Data.SupplierItems {
		if !strings.EqualFold(strings.TrimSpace(row.Supplier), strings.TrimSpace(supplierLink)) {
			continue
		}
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		deleteEndpoint := normalized + "/api/resource/Item%20Supplier/" + url.PathEscape(strings.TrimSpace(row.Name))
		if err := c.doJSONRequest(ctx, http.MethodDelete, deleteEndpoint, apiKey, apiSecret, nil, nil); err != nil {
			return err
		}
	}

	if strings.EqualFold(strings.TrimSpace(payload.Data.DefaultSupplier), strings.TrimSpace(supplierLink)) {
		if err := c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
			"default_supplier": "",
		}, nil); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]Item, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, itemCodes, "", len(itemCodes))
}

func (c *Client) CreateDraftPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret string, input CreatePurchaseReceiptInput) (PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}
	if input.Qty <= 0 {
		return PurchaseReceiptDraft{}, fmt.Errorf("qty must be greater than 0")
	}
	if strings.TrimSpace(input.ItemCode) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("item code is required")
	}
	if strings.TrimSpace(input.Supplier) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("supplier is required")
	}
	if strings.TrimSpace(input.Warehouse) == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("warehouse is required")
	}

	supplierLink, err := c.resolveSupplierLink(ctx, normalized, apiKey, apiSecret, input.Supplier)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	company, err := c.fetchWarehouseCompany(ctx, normalized, apiKey, apiSecret, input.Warehouse)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	uom := strings.TrimSpace(input.UOM)
	if uom == "" {
		items, err := c.searchItemsByCodes(ctx, normalized, apiKey, apiSecret, []string{input.ItemCode}, input.ItemCode, 1)
		if err != nil {
			return PurchaseReceiptDraft{}, err
		}
		if len(items) > 0 && strings.TrimSpace(items[0].UOM) != "" {
			uom = strings.TrimSpace(items[0].UOM)
		}
	}
	if uom == "" {
		uom = "Nos"
	}

	payload := map[string]interface{}{
		"supplier":               supplierLink,
		"company":                company,
		"posting_date":           time.Now().Format("2006-01-02"),
		"set_warehouse":          strings.TrimSpace(input.Warehouse),
		"supplier_delivery_note": buildTelegramReceiptMarker(input.SupplierPhone, input.Qty, time.Now().UTC()),
		"items": []map[string]interface{}{
			{
				"item_code":                 strings.TrimSpace(input.ItemCode),
				"warehouse":                 strings.TrimSpace(input.Warehouse),
				"qty":                       input.Qty,
				"received_qty":              input.Qty,
				"uom":                       uom,
				"stock_uom":                 uom,
				"conversion_factor":         1,
				"stock_qty":                 input.Qty,
				"received_stock_qty":        input.Qty,
				"rate":                      0,
				"allow_zero_valuation_rate": 1,
			},
		},
	}

	var createResp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt"
	if err := c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, payload, &createResp); err != nil {
		return PurchaseReceiptDraft{}, err
	}
	if createResp.Data.Name == "" {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt create response did not return name")
	}

	return c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, createResp.Data.Name)
}

func (c *Client) ListPendingPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]PurchaseReceiptDraft, error) {
	return c.ListPendingPurchaseReceiptsPage(ctx, baseURL, apiKey, apiSecret, limit, 0)
}

func (c *Client) ListPendingPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"docstatus", "=", 0},
	})
	return c.listPurchaseReceipts(ctx, normalized, apiKey, apiSecret, filtersJSON, limit, offset)
}

func (c *Client) ListSupplierPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit int) ([]PurchaseReceiptDraft, error) {
	return c.ListSupplierPurchaseReceiptsPage(ctx, baseURL, apiKey, apiSecret, supplier, limit, 0)
}

func (c *Client) ListSupplierPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret, supplier string, limit, offset int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier", "=", strings.TrimSpace(supplier)},
		{"supplier_delivery_note", "like", telegramReceiptMarkerPrefix + "%"},
	})
	return c.listPurchaseReceipts(ctx, normalized, apiKey, apiSecret, filtersJSON, limit, offset)
}

func (c *Client) ListTelegramPurchaseReceipts(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]PurchaseReceiptDraft, error) {
	return c.ListTelegramPurchaseReceiptsPage(ctx, baseURL, apiKey, apiSecret, limit, 0)
}

func (c *Client) ListTelegramPurchaseReceiptsPage(ctx context.Context, baseURL, apiKey, apiSecret string, limit, offset int) ([]PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier_delivery_note", "like", telegramReceiptMarkerPrefix + "%"},
	})
	return c.listPurchaseReceipts(ctx, normalized, apiKey, apiSecret, filtersJSON, limit, offset)
}

func (c *Client) listPurchaseReceipts(ctx context.Context, normalized, apiKey, apiSecret string, filtersJSON []byte, limit, offset int) ([]PurchaseReceiptDraft, error) {
	params := url.Values{}
	params.Set("fields", `["name","supplier","supplier_name","posting_date","supplier_delivery_note","status","docstatus","currency","remarks","items"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	if offset > 0 {
		params.Set("limit_start", fmt.Sprintf("%d", offset))
	}
	params.Set("order_by", "modified desc")

	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase Receipt?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]PurchaseReceiptDraft, 0, len(payload.Data))
	for _, row := range payload.Data {
		doc, err := mapPurchaseReceiptDraft(row)
		if err != nil {
			name := strings.TrimSpace(getStringValue(row["name"]))
			if name == "" {
				return nil, err
			}
			doc, err = c.GetPurchaseReceipt(ctx, normalized, apiKey, apiSecret, name)
			if err != nil {
				return nil, err
			}
		}
		items = append(items, doc)
	}
	return items, nil
}

func (c *Client) GetPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string) (PurchaseReceiptDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	doc, err := c.fetchPurchaseReceiptDoc(ctx, normalized, apiKey, apiSecret, name)
	if err != nil {
		return PurchaseReceiptDraft{}, err
	}

	return mapPurchaseReceiptDraft(doc)
}

func (c *Client) ListPurchaseReceiptComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]Comment, error) {
	itemsByName, err := c.ListPurchaseReceiptCommentsBatch(ctx, baseURL, apiKey, apiSecret, []string{name}, limit)
	if err != nil {
		return nil, err
	}
	return itemsByName[strings.TrimSpace(name)], nil
}

func (c *Client) ListPurchaseReceiptCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]Comment, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	normalizedNames := make([]string, 0, len(names))
	seenNames := make(map[string]struct{}, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seenNames[trimmed]; ok {
			continue
		}
		seenNames[trimmed] = struct{}{}
		normalizedNames = append(normalizedNames, trimmed)
	}
	if len(normalizedNames) == 0 {
		return map[string][]Comment{}, nil
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"reference_doctype", "=", "Purchase Receipt"},
		{"reference_name", "in", normalizedNames},
		{"comment_type", "=", "Comment"},
	})
	params := url.Values{}
	params.Set("fields", `["name","content","creation","reference_name"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("order_by", "reference_name asc, creation asc")
	params.Set("limit_page_length", fmt.Sprintf("%d", len(normalizedNames)*limit))

	var payload struct {
		Data []struct {
			Name          string `json:"name"`
			Content       string `json:"content"`
			Creation      string `json:"creation"`
			ReferenceName string `json:"reference_name"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Comment?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	itemsByName := make(map[string][]Comment, len(normalizedNames))
	for _, row := range payload.Data {
		name := strings.TrimSpace(row.ReferenceName)
		if name == "" {
			continue
		}
		if len(itemsByName[name]) >= limit {
			continue
		}
		itemsByName[name] = append(itemsByName[name], Comment{
			ID:        strings.TrimSpace(row.Name),
			Content:   strings.TrimSpace(row.Content),
			CreatedAt: strings.TrimSpace(row.Creation),
		})
	}
	for _, name := range normalizedNames {
		if _, ok := itemsByName[name]; !ok {
			itemsByName[name] = []Comment{}
		}
	}
	return itemsByName, nil
}

func (c *Client) AddPurchaseReceiptComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	return c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, content)
}

func (c *Client) ConfirmAndSubmitPurchaseReceipt(ctx context.Context, baseURL, apiKey, apiSecret, name string, acceptedQty, returnedQty float64, returnReason, returnComment string) (PurchaseReceiptSubmissionResult, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	if acceptedQty < 0 {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("accepted qty cannot be negative")
	}

	doc, err := c.fetchPurchaseReceiptDoc(ctx, normalized, apiKey, apiSecret, name)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	originalDoc := cloneDocumentMap(doc)

	draft, err := mapPurchaseReceiptDraft(doc)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	if acceptedQty > draft.Qty {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("accepted qty cannot exceed sent qty")
	}
	decisionNote, err := buildAccordDecisionNote(draft, acceptedQty, returnedQty, returnReason, returnComment)
	if err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}
	fullReturn := acceptedQty == 0 && returnedQty >= draft.Qty && draft.Qty > 0

	if fullReturn {
		if strings.TrimSpace(decisionNote) != "" {
			updateEndpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(name)
			if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, map[string]string{
				"remarks": upsertAccordDecisionInRemarks(strings.TrimSpace(draft.Remarks), decisionNote),
			}, nil); err != nil {
				return PurchaseReceiptSubmissionResult{}, err
			}
			_ = c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, decisionNote)
		}
		return PurchaseReceiptSubmissionResult{
			Name:                 name,
			Supplier:             draft.Supplier,
			ItemCode:             draft.ItemCode,
			UOM:                  draft.UOM,
			SentQty:              draft.Qty,
			AcceptedQty:          0,
			SupplierDeliveryNote: draft.SupplierDeliveryNote,
			Note:                 ExtractAccordDecisionNote(decisionNote),
		}, nil
	}

	items, ok := doc["items"].([]interface{})
	if !ok || len(items) == 0 {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("purchase receipt %s has no items", name)
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return PurchaseReceiptSubmissionResult{}, fmt.Errorf("purchase receipt %s item payload is invalid", name)
	}

	conversionFactor := getFloatValue(firstItem["conversion_factor"])
	if conversionFactor <= 0 {
		conversionFactor = 1
	}
	stockQty := acceptedQty * conversionFactor
	receivedQty := acceptedQty + returnedQty
	receivedStockQty := receivedQty * conversionFactor

	firstItem["qty"] = acceptedQty
	firstItem["received_qty"] = receivedQty
	firstItem["stock_qty"] = stockQty
	firstItem["received_stock_qty"] = receivedStockQty
	firstItem["rejected_qty"] = returnedQty
	if returnedQty > 0 {
		rejectedWarehouse := strings.TrimSpace(getStringValue(firstItem["rejected_warehouse"]))
		acceptedWarehouse := strings.TrimSpace(getStringValue(firstItem["warehouse"]))
		if rejectedWarehouse == "" || strings.EqualFold(rejectedWarehouse, acceptedWarehouse) {
			rejectedWarehouse, err = c.findAlternateWarehouse(ctx, normalized, apiKey, apiSecret, acceptedWarehouse)
			if err != nil {
				return PurchaseReceiptSubmissionResult{}, err
			}
		}
		firstItem["rejected_warehouse"] = rejectedWarehouse
	} else {
		firstItem["rejected_warehouse"] = ""
	}
	firstItem["allow_zero_valuation_rate"] = 1
	if _, ok := firstItem["rate"]; !ok {
		firstItem["rate"] = 0
	}
	if strings.TrimSpace(decisionNote) != "" {
		doc["remarks"] = upsertAccordDecisionInRemarks(getStringValue(doc["remarks"]), decisionNote)
	}

	updateEndpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(name)
	if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, doc, nil); err != nil {
		return PurchaseReceiptSubmissionResult{}, err
	}

	if err := c.submitDoc(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name); err != nil {
		if rollbackErr := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, originalDoc, nil); rollbackErr != nil {
			return PurchaseReceiptSubmissionResult{}, fmt.Errorf("submit failed: %v; rollback failed: %v", err, rollbackErr)
		}
		return PurchaseReceiptSubmissionResult{}, err
	}
	if strings.TrimSpace(decisionNote) != "" {
		_ = c.addComment(ctx, normalized, apiKey, apiSecret, "Purchase Receipt", name, decisionNote)
	}

	return PurchaseReceiptSubmissionResult{
		Name:                 name,
		Supplier:             draft.Supplier,
		ItemCode:             draft.ItemCode,
		UOM:                  draft.UOM,
		SentQty:              draft.Qty,
		AcceptedQty:          acceptedQty,
		SupplierDeliveryNote: draft.SupplierDeliveryNote,
		Note:                 ExtractAccordDecisionNote(decisionNote),
	}, nil
}

func cloneDocumentMap(input map[string]interface{}) map[string]interface{} {
	raw, err := json.Marshal(input)
	if err != nil {
		return input
	}
	var cloned map[string]interface{}
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return input
	}
	return cloned
}

func (c *Client) addComment(ctx context.Context, normalized, apiKey, apiSecret, doctype, name, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	endpoint := normalized + "/api/resource/Comment"
	return c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, map[string]string{
		"comment_type":      "Comment",
		"reference_doctype": strings.TrimSpace(doctype),
		"reference_name":    strings.TrimSpace(name),
		"content":           strings.TrimSpace(content),
	}, nil)
}

func (c *Client) resolveSupplierLink(ctx context.Context, normalized, apiKey, apiSecret, supplier string) (string, error) {
	links, err := c.searchLink(ctx, normalized, apiKey, apiSecret, "Supplier", supplier, 5)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", fmt.Errorf("supplier not found: %s", supplier)
	}

	needle := strings.TrimSpace(strings.ToLower(supplier))
	for _, item := range links {
		if strings.TrimSpace(strings.ToLower(item)) == needle {
			return item, nil
		}
	}
	return links[0], nil
}

func (c *Client) fetchSupplierItemCodes(ctx context.Context, normalized, apiKey, apiSecret, supplier string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"supplier", "=", supplier},
	})
	params := url.Values{}
	params.Set("parent", "Item")
	params.Set("fields", `["parent"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))

	var payload struct {
		Data []struct {
			Parent string `json:"parent"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item%20Supplier?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(payload.Data))
	for _, row := range payload.Data {
		code := strings.TrimSpace(row.Parent)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result, nil
}

func (c *Client) itemHasSupplier(ctx context.Context, normalized, apiKey, apiSecret, itemCode, supplier string) (bool, error) {
	var payload struct {
		Data struct {
			DefaultSupplier string `json:"default_supplier"`
			SupplierItems   []struct {
				Supplier string `json:"supplier"`
			} `json:"supplier_items"`
		} `json:"data"`
	}

	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return false, err
	}

	if strings.EqualFold(strings.TrimSpace(payload.Data.DefaultSupplier), strings.TrimSpace(supplier)) {
		return true, nil
	}
	for _, row := range payload.Data.SupplierItems {
		if strings.EqualFold(strings.TrimSpace(row.Supplier), strings.TrimSpace(supplier)) {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) searchItemsByCodes(ctx context.Context, normalized, apiKey, apiSecret string, itemCodes []string, query string, limit int) ([]Item, error) {
	if len(itemCodes) == 0 {
		return []Item{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	if len(itemCodes) > 500 {
		itemCodes = itemCodes[:500]
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
		{"is_stock_item", "=", 1},
		{"name", "in", itemCodes},
	})

	params := url.Values{}
	params.Set("fields", `["name","item_name","stock_uom"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", len(itemCodes)))

	var payload struct {
		Data []struct {
			Name     string `json:"name"`
			ItemName string `json:"item_name"`
			StockUOM string `json:"stock_uom"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Item?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(payload.Data))
	for _, row := range payload.Data {
		displayName := row.ItemName
		if displayName == "" {
			displayName = row.Name
		}
		items = append(items, Item{
			Code: row.Name,
			Name: displayName,
			UOM:  row.StockUOM,
		})
	}
	if strings.TrimSpace(query) != "" {
		filtered := make([]Item, 0, len(items))
		for _, item := range items {
			if SearchQueryScore(query, item.Code, item.Name) == 0 {
				continue
			}
			filtered = append(filtered, item)
		}
		items = filtered
		sort.Slice(items, func(i, j int) bool {
			leftScore := SearchQueryScore(query, items[i].Code, items[i].Name)
			rightScore := SearchQueryScore(query, items[j].Code, items[j].Name)
			if leftScore != rightScore {
				return leftScore > rightScore
			}
			leftName := strings.ToLower(strings.TrimSpace(items[i].Name))
			rightName := strings.ToLower(strings.TrimSpace(items[j].Name))
			if leftName != rightName {
				return leftName < rightName
			}
			return strings.ToLower(strings.TrimSpace(items[i].Code)) < strings.ToLower(strings.TrimSpace(items[j].Code))
		})
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *Client) fetchWarehouseCompany(ctx context.Context, normalized, apiKey, apiSecret, warehouse string) (string, error) {
	var payload struct {
		Data struct {
			Company string `json:"company"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Warehouse/" + url.PathEscape(strings.TrimSpace(warehouse))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Data.Company) == "" {
		return "", fmt.Errorf("company not found for warehouse %s", warehouse)
	}
	return payload.Data.Company, nil
}

func (c *Client) findAlternateWarehouse(ctx context.Context, normalized, apiKey, apiSecret, acceptedWarehouse string) (string, error) {
	params := url.Values{}
	params.Set("fields", `["name","is_group"]`)
	params.Set("limit_page_length", "50")

	var payload struct {
		Data []struct {
			Name    string `json:"name"`
			IsGroup int    `json:"is_group"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Warehouse?" + params.Encode()
	err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload)
	if err != nil {
		return "", err
	}
	for _, item := range payload.Data {
		name := strings.TrimSpace(item.Name)
		if item.IsGroup != 0 {
			continue
		}
		if name == "" || strings.EqualFold(name, strings.TrimSpace(acceptedWarehouse)) {
			continue
		}
		return name, nil
	}
	return "", fmt.Errorf("rejected warehouse topilmadi: %s", acceptedWarehouse)
}

func (c *Client) fetchPurchaseReceiptDoc(ctx context.Context, normalized, apiKey, apiSecret, name string) (map[string]interface{}, error) {
	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("purchase receipt %s not found", name)
	}
	return payload.Data, nil
}

func (c *Client) submitDoc(ctx context.Context, normalized, apiKey, apiSecret, doctype, name string) error {
	submitPayload := map[string]interface{}{
		"doc": map[string]interface{}{},
	}
	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	docEndpoint := normalized + "/api/resource/" + url.PathEscape(doctype) + "/" + url.PathEscape(name)

	for attempt := 0; attempt < 2; attempt++ {
		var latest struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := c.doJSON(ctx, docEndpoint, apiKey, apiSecret, &latest); err != nil {
			return err
		}
		if len(latest.Data) == 0 {
			return fmt.Errorf("%s %s not found before submit", doctype, name)
		}
		submitPayload["doc"] = latest.Data

		if err := c.doJSONRequest(ctx, http.MethodPost, submitEndpoint, apiKey, apiSecret, submitPayload, nil); err != nil {
			if attempt == 0 && strings.Contains(err.Error(), "TimestampMismatchError") {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

func mapPurchaseReceiptDraft(doc map[string]interface{}) (PurchaseReceiptDraft, error) {
	items, ok := doc["items"].([]interface{})
	if !ok || len(items) == 0 {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt has no items")
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return PurchaseReceiptDraft{}, fmt.Errorf("purchase receipt item payload is invalid")
	}

	itemCode := getStringValue(firstItem["item_code"])
	itemName := getStringValue(firstItem["item_name"])
	if itemName == "" {
		itemName = itemCode
	}

	uom := getStringValue(firstItem["uom"])
	if uom == "" {
		uom = getStringValue(firstItem["stock_uom"])
	}

	qty := getFloatValue(firstItem["qty"])
	if markerQty, ok := ParseTelegramReceiptMarkerQty(getStringValue(doc["supplier_delivery_note"])); ok && markerQty > qty {
		qty = markerQty
	}

	return PurchaseReceiptDraft{
		Name:                 getStringValue(doc["name"]),
		DocStatus:            int(getFloatValue(doc["docstatus"])),
		Status:               getStringValue(doc["status"]),
		Supplier:             getStringValue(doc["supplier"]),
		SupplierName:         getStringValue(doc["supplier_name"]),
		PostingDate:          getStringValue(doc["posting_date"]),
		SupplierDeliveryNote: getStringValue(doc["supplier_delivery_note"]),
		ItemCode:             itemCode,
		ItemName:             itemName,
		Qty:                  qty,
		UOM:                  uom,
		Warehouse:            getStringValue(firstItem["warehouse"]),
		Amount:               getFloatValue(firstItem["amount"]),
		Currency:             getStringValue(doc["currency"]),
		Remarks:              getStringValue(doc["remarks"]),
	}, nil
}

const (
	accordAcceptedLinePrefix           = "Accord Qabul:"
	accordReturnedLinePrefix           = "Accord Qaytarildi:"
	accordReasonLinePrefix             = "Accord Sabab:"
	accordCommentLinePrefix            = "Accord Izoh:"
	accordSupplierAckPrefix            = "Accord Supplier Tasdiq:"
	accordWerkaUnannouncedPrefix       = "Accord Werka Aytilmagan:"
	accordWerkaUnannouncedReasonPrefix = "Accord Werka Aytilmagan Sabab:"
)

func buildAccordDecisionNote(draft PurchaseReceiptDraft, acceptedQty, returnedQty float64, returnReason, returnComment string) (string, error) {
	impliedReturnedQty := draft.Qty - acceptedQty
	if impliedReturnedQty < 0 {
		impliedReturnedQty = 0
	}
	trimmedComment := strings.TrimSpace(returnComment)
	if impliedReturnedQty <= 0 {
		return "", nil
	}

	if returnedQty < 0 {
		return "", fmt.Errorf("returned qty cannot be negative")
	}
	if returnedQty == 0 {
		returnedQty = impliedReturnedQty
	}
	if returnedQty-impliedReturnedQty > 0.0001 {
		return "", fmt.Errorf("returned qty cannot exceed sent minus accepted qty")
	}

	lines := []string{
		fmt.Sprintf("%s %.4f %s", accordAcceptedLinePrefix, acceptedQty, draft.UOM),
		fmt.Sprintf("%s %.4f %s", accordReturnedLinePrefix, returnedQty, draft.UOM),
	}
	if strings.TrimSpace(returnReason) != "" {
		lines = append(lines, accordReasonLinePrefix+" "+strings.TrimSpace(returnReason))
	}
	if trimmedComment != "" {
		lines = append(lines, accordCommentLinePrefix+" "+trimmedComment)
	}
	return strings.Join(lines, "\n"), nil
}

func upsertAccordDecisionInRemarks(existing, decision string) string {
	lines := strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, accordAcceptedLinePrefix) ||
			strings.HasPrefix(trimmed, accordReturnedLinePrefix) ||
			strings.HasPrefix(trimmed, accordReasonLinePrefix) ||
			strings.HasPrefix(trimmed, accordCommentLinePrefix) ||
			strings.HasPrefix(trimmed, accordSupplierAckPrefix) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if strings.TrimSpace(decision) != "" {
		filtered = append(filtered, strings.Split(strings.TrimSpace(decision), "\n")...)
	}
	return strings.Join(filtered, "\n")
}

func ExtractAccordDecisionNote(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	result := make([]string, 0, 3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, accordAcceptedLinePrefix):
			result = append(result, "Qabul: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordAcceptedLinePrefix)))
		case strings.HasPrefix(trimmed, accordReturnedLinePrefix):
			result = append(result, "Qaytarildi: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordReturnedLinePrefix)))
		case strings.HasPrefix(trimmed, accordReasonLinePrefix):
			result = append(result, "Sabab: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordReasonLinePrefix)))
		case strings.HasPrefix(trimmed, accordCommentLinePrefix):
			result = append(result, "Izoh: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordCommentLinePrefix)))
		case strings.HasPrefix(trimmed, accordSupplierAckPrefix):
			result = append(result, "Supplier tasdiqladi: "+strings.TrimSpace(strings.TrimPrefix(trimmed, accordSupplierAckPrefix)))
		}
	}
	return strings.Join(result, "\n")
}

func ExtractAccordDecisionQuantities(remarks string) (acceptedQty, returnedQty float64) {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, accordAcceptedLinePrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, accordAcceptedLinePrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				acceptedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		case strings.HasPrefix(trimmed, accordReturnedLinePrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, accordReturnedLinePrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				returnedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		}
	}
	return acceptedQty, returnedQty
}

func UpsertSupplierAcknowledgmentInRemarks(existingNote, message string) string {
	lines := strings.Split(strings.ReplaceAll(existingNote, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Supplier tasdiqladi:") {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	filtered = append(filtered, accordSupplierAckPrefix+" "+strings.TrimSpace(message))
	return strings.Join(filtered, "\n")
}

func UpsertWerkaUnannouncedInRemarks(existingNote, state, reason string) string {
	lines := strings.Split(strings.ReplaceAll(existingNote, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, accordWerkaUnannouncedPrefix) ||
			strings.HasPrefix(trimmed, accordWerkaUnannouncedReasonPrefix) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if strings.TrimSpace(state) != "" {
		filtered = append(filtered, accordWerkaUnannouncedPrefix+" "+strings.TrimSpace(state))
	}
	if strings.TrimSpace(reason) != "" {
		filtered = append(filtered, accordWerkaUnannouncedReasonPrefix+" "+strings.TrimSpace(reason))
	}
	return strings.Join(filtered, "\n")
}

func ExtractWerkaUnannouncedState(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, accordWerkaUnannouncedPrefix) {
			return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, accordWerkaUnannouncedPrefix)))
		}
	}
	return ""
}

func ExtractWerkaUnannouncedReason(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, accordWerkaUnannouncedReasonPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, accordWerkaUnannouncedReasonPrefix))
		}
	}
	return ""
}

func (c *Client) UpdatePurchaseReceiptRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Purchase%20Receipt/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
		"remarks": strings.TrimSpace(remarks),
	}, nil)
}

func buildTelegramReceiptMarker(phone string, qty float64, now time.Time) string {
	normalizedPhone := strings.TrimSpace(phone)
	if normalizedPhone == "" {
		normalizedPhone = "unknown"
	}
	return fmt.Sprintf("%s%s:%s:%.4f", telegramReceiptMarkerPrefix, normalizedPhone, now.Format("20060102150405"), qty)
}

func ParseTelegramReceiptMarkerQty(marker string) (float64, bool) {
	trimmed := strings.TrimSpace(marker)
	if !strings.HasPrefix(trimmed, telegramReceiptMarkerPrefix) {
		return 0, false
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) < 4 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(parts[len(parts)-1]), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func getStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.Itoa(int(typed))
	case float32:
		return strconv.Itoa(int(typed))
	case int:
		return strconv.Itoa(typed)
	case int32:
		return strconv.Itoa(int(typed))
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func getFloatValue(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) SearchCompanies(ctx context.Context, baseURL, apiKey, apiSecret string, limit int) ([]Company, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	endpoint := normalized + "/api/resource/Company?fields=%5B%22name%22%5D&limit_page_length=" + fmt.Sprintf("%d", limit)
	var payload struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Company, 0, len(payload.Data))
	for _, row := range payload.Data {
		items = append(items, Company{Name: strings.TrimSpace(row.Name)})
	}
	return items, nil
}

func (c *Client) CreateAndSubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateDeliveryNoteInput) (DeliveryNoteResult, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return DeliveryNoteResult{}, err
	}
	if input.Qty <= 0 {
		return DeliveryNoteResult{}, fmt.Errorf("qty must be greater than 0")
	}
	if strings.TrimSpace(input.Customer) == "" {
		return DeliveryNoteResult{}, fmt.Errorf("customer is required")
	}
	if strings.TrimSpace(input.Company) == "" {
		return DeliveryNoteResult{}, fmt.Errorf("company is required")
	}
	if strings.TrimSpace(input.Warehouse) == "" {
		return DeliveryNoteResult{}, fmt.Errorf("warehouse is required")
	}
	if strings.TrimSpace(input.ItemCode) == "" {
		return DeliveryNoteResult{}, fmt.Errorf("item code is required")
	}
	if strings.TrimSpace(input.UOM) == "" {
		input.UOM = "Nos"
	}

	payload := map[string]interface{}{
		"customer":      strings.TrimSpace(input.Customer),
		"company":       strings.TrimSpace(input.Company),
		"set_warehouse": strings.TrimSpace(input.Warehouse),
		"items": []map[string]interface{}{
			{
				"item_code":         strings.TrimSpace(input.ItemCode),
				"qty":               input.Qty,
				"uom":               strings.TrimSpace(input.UOM),
				"stock_uom":         strings.TrimSpace(input.UOM),
				"conversion_factor": 1,
				"warehouse":         strings.TrimSpace(input.Warehouse),
			},
		},
	}

	var createResp struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	createEndpoint := normalized + "/api/resource/Delivery%20Note"
	if err := c.doJSONRequest(ctx, http.MethodPost, createEndpoint, apiKey, apiSecret, payload, &createResp); err != nil {
		return DeliveryNoteResult{}, err
	}
	if createResp.Data.Name == "" {
		return DeliveryNoteResult{}, fmt.Errorf("delivery note create response did not return name")
	}

	submitPayload := map[string]interface{}{
		"doc": map[string]interface{}{},
	}
	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	docEndpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(createResp.Data.Name)
	for attempt := 0; attempt < 2; attempt++ {
		var latest struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := c.doJSON(ctx, docEndpoint, apiKey, apiSecret, &latest); err != nil {
			return DeliveryNoteResult{}, err
		}
		if len(latest.Data) == 0 {
			return DeliveryNoteResult{}, fmt.Errorf("delivery note %s not found after create", createResp.Data.Name)
		}
		submitPayload["doc"] = latest.Data

		if err := c.doJSONRequest(ctx, http.MethodPost, submitEndpoint, apiKey, apiSecret, submitPayload, nil); err != nil {
			if attempt == 0 && strings.Contains(err.Error(), "TimestampMismatchError") {
				continue
			}
			return DeliveryNoteResult{}, err
		}
		break
	}

	return DeliveryNoteResult{Name: createResp.Data.Name}, nil
}

func (c *Client) ListCustomerDeliveryNotes(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit int) ([]DeliveryNoteDraft, error) {
	return c.ListCustomerDeliveryNotesPage(ctx, baseURL, apiKey, apiSecret, customer, limit, 0)
}

func (c *Client) ListCustomerDeliveryNotesPage(ctx context.Context, baseURL, apiKey, apiSecret, customer string, limit, offset int) ([]DeliveryNoteDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"customer", "=", strings.TrimSpace(customer)},
	})

	params := url.Values{}
	params.Set("fields", `["name","customer","customer_name","posting_date","status","docstatus","items"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", fmt.Sprintf("%d", limit))
	if offset > 0 {
		params.Set("limit_start", fmt.Sprintf("%d", offset))
	}
	params.Set("order_by", "modified desc")

	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Delivery%20Note?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]DeliveryNoteDraft, 0, len(payload.Data))
	for _, row := range payload.Data {
		doc, err := mapDeliveryNoteDraft(row)
		if err != nil {
			return nil, err
		}
		if doc.ItemCode == "" || doc.ItemName == "" || doc.Qty <= 0 {
			full, err := c.GetDeliveryNote(ctx, normalized, apiKey, apiSecret, doc.Name)
			if err != nil {
				return nil, err
			}
			doc = full
		}
		items = append(items, doc)
	}
	return items, nil
}

func (c *Client) GetDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) (DeliveryNoteDraft, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return DeliveryNoteDraft{}, err
	}
	endpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(strings.TrimSpace(name))
	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return DeliveryNoteDraft{}, err
	}
	return mapDeliveryNoteDraft(payload.Data)
}

func mapDeliveryNoteDraft(doc map[string]interface{}) (DeliveryNoteDraft, error) {
	result := DeliveryNoteDraft{
		Name:         getStringValue(doc["name"]),
		Customer:     getStringValue(doc["customer"]),
		CustomerName: getStringValue(doc["customer_name"]),
		PostingDate:  getStringValue(doc["posting_date"]),
		Status:       getStringValue(doc["status"]),
		DocStatus:    int(getFloatValue(doc["docstatus"])),
	}
	items, _ := doc["items"].([]interface{})
	if len(items) == 0 {
		return result, nil
	}
	firstItem, _ := items[0].(map[string]interface{})
	result.ItemCode = getStringValue(firstItem["item_code"])
	result.ItemName = getStringValue(firstItem["item_name"])
	result.Qty = getFloatValue(firstItem["qty"])
	result.UOM = getStringValue(firstItem["uom"])
	if result.UOM == "" {
		result.UOM = getStringValue(firstItem["stock_uom"])
	}
	return result, nil
}

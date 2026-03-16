package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	compactCustomerDecisionPrefix      = "AC:"
	compactCustomerReasonPrefix        = "AR:"
	compactDeliveryLifecyclePrefix     = "AD:"
	compactDeliveryActorPrefix         = "AA:"
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

func (c *Client) CreateDraftDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateDeliveryNoteInput) (DeliveryNoteResult, error) {
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
	if strings.TrimSpace(input.Remarks) != "" {
		payload["remarks"] = strings.TrimSpace(input.Remarks)
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
	return DeliveryNoteResult{Name: createResp.Data.Name}, nil
}

func (c *Client) SubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	submitPayload := map[string]interface{}{
		"doc": map[string]interface{}{},
	}
	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	docEndpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(strings.TrimSpace(name))
	for attempt := 0; attempt < 2; attempt++ {
		var latest struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := c.doJSON(ctx, docEndpoint, apiKey, apiSecret, &latest); err != nil {
			return err
		}
		if len(latest.Data) == 0 {
			return fmt.Errorf("delivery note %s not found after create", strings.TrimSpace(name))
		}
		submitPayload["doc"] = latest.Data

		if err := c.doJSONRequest(ctx, http.MethodPost, submitEndpoint, apiKey, apiSecret, submitPayload, nil); err != nil {
			if attempt == 0 && strings.Contains(err.Error(), "TimestampMismatchError") {
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (c *Client) CreateAndSubmitDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateDeliveryNoteInput) (DeliveryNoteResult, error) {
	result, err := c.CreateDraftDeliveryNote(ctx, baseURL, apiKey, apiSecret, input)
	if err != nil {
		return DeliveryNoteResult{}, err
	}
	if err := c.SubmitDeliveryNote(ctx, baseURL, apiKey, apiSecret, result.Name); err != nil {
		return DeliveryNoteResult{}, err
	}
	return result, nil
}

func (c *Client) UpdateDeliveryNoteRemarks(ctx context.Context, baseURL, apiKey, apiSecret, name, remarks string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
		"remarks": strings.TrimSpace(remarks),
	}, nil)
}

func (c *Client) ListDeliveryNoteComments(ctx context.Context, baseURL, apiKey, apiSecret, name string, limit int) ([]Comment, error) {
	itemsByName, err := c.ListDeliveryNoteCommentsBatch(ctx, baseURL, apiKey, apiSecret, []string{name}, limit)
	if err != nil {
		return nil, err
	}
	return itemsByName[strings.TrimSpace(name)], nil
}

func (c *Client) ListDeliveryNoteCommentsBatch(ctx context.Context, baseURL, apiKey, apiSecret string, names []string, limit int) (map[string][]Comment, error) {
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
		{"reference_doctype", "=", "Delivery Note"},
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

func (c *Client) AddDeliveryNoteComment(ctx context.Context, baseURL, apiKey, apiSecret, name, content string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	return c.addComment(ctx, normalized, apiKey, apiSecret, "Delivery Note", name, content)
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
		if doc.ItemCode == "" || doc.ItemName == "" || doc.Qty <= 0 || doc.DocStatus == 0 {
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
		Remarks:      getStringValue(doc["remarks"]),
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

func UpsertCustomerDecisionInRemarks(existingNote, state, reason string) string {
	lines := strings.Split(strings.ReplaceAll(existingNote, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+2)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, compactCustomerDecisionPrefix) ||
			strings.HasPrefix(trimmed, compactCustomerReasonPrefix) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if normalized := normalizeCustomerDecisionState(state); normalized != "" {
		filtered = append(filtered, compactCustomerDecisionPrefix+normalized)
	}
	if strings.TrimSpace(reason) != "" {
		filtered = append(filtered, compactCustomerReasonPrefix+strings.TrimSpace(reason))
	}
	return strings.Join(filtered, "\n")
}

func ExtractCustomerDecisionState(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, compactCustomerDecisionPrefix) {
			return normalizeCustomerDecisionState(strings.TrimSpace(strings.TrimPrefix(trimmed, compactCustomerDecisionPrefix)))
		}
	}
	return ""
}

func ExtractCustomerDecisionReason(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, compactCustomerReasonPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, compactCustomerReasonPrefix))
		}
	}
	return ""
}

func BuildDeliveryLifecycleComment(state, actor string) string {
	lines := make([]string, 0, 2)
	if normalized := normalizeDeliveryLifecycleState(state); normalized != "" {
		lines = append(lines, compactDeliveryLifecyclePrefix+normalized)
	}
	if strings.TrimSpace(actor) != "" {
		lines = append(lines, compactDeliveryActorPrefix+normalizeDeliveryActor(actor))
	}
	return strings.Join(lines, "\n")
}

func ExtractDeliveryLifecycleState(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, compactDeliveryLifecyclePrefix) {
			return normalizeDeliveryLifecycleState(strings.TrimSpace(strings.TrimPrefix(trimmed, compactDeliveryLifecyclePrefix)))
		}
	}
	return ""
}

func normalizeCustomerDecisionState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "pending", "pd":
		return "pending"
	case "confirmed", "accepted", "cf":
		return "confirmed"
	case "rejected", "rj":
		return "rejected"
	default:
		return ""
	}
}

func normalizeDeliveryLifecycleState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "submitted", "sb":
		return "submitted"
	default:
		return ""
	}
}

func normalizeDeliveryActor(actor string) string {
	switch strings.ToLower(strings.TrimSpace(actor)) {
	case "werka", "wk":
		return "wk"
	case "customer", "cu":
		return "cu"
	case "admin", "ad":
		return "ad"
	default:
		return strings.ToLower(strings.TrimSpace(actor))
	}
}

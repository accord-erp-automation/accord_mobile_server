package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	compactCustomerDecisionPrefix  = "AC:"
	compactCustomerReasonPrefix    = "AR:"
	compactCustomerAcceptedPrefix  = "AQ:"
	compactCustomerReturnedPrefix  = "AT:"
	compactCustomerCommentPrefix   = "AX:"
	compactDeliveryLifecyclePrefix = "AD:"
	compactDeliveryActorPrefix     = "AA:"
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
	if err := c.EnsureDeliveryNoteStateFields(ctx, baseURL, apiKey, apiSecret); err != nil {
		return DeliveryNoteResult{}, err
	}
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

func (c *Client) CreateAndSubmitDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string) (DeliveryNoteResult, error) {
	return c.createAndSubmitDeliveryNoteReturnWithQty(ctx, baseURL, apiKey, apiSecret, sourceName, 0)
}

func (c *Client) CreateAndSubmitPartialDeliveryNoteReturn(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, returnedQty float64) (DeliveryNoteResult, error) {
	if returnedQty <= 0 {
		return DeliveryNoteResult{}, fmt.Errorf("returned qty must be greater than 0")
	}
	return c.createAndSubmitDeliveryNoteReturnWithQty(ctx, baseURL, apiKey, apiSecret, sourceName, returnedQty)
}

func (c *Client) createAndSubmitDeliveryNoteReturnWithQty(ctx context.Context, baseURL, apiKey, apiSecret, sourceName string, returnedQty float64) (DeliveryNoteResult, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return DeliveryNoteResult{}, err
	}

	makeEndpoint := normalized + "/api/method/erpnext.stock.doctype.delivery_note.delivery_note.make_sales_return?source_name=" + url.QueryEscape(strings.TrimSpace(sourceName))
	var mapped struct {
		Message map[string]interface{} `json:"message"`
	}
	if err := c.doJSON(ctx, makeEndpoint, apiKey, apiSecret, &mapped); err != nil {
		return DeliveryNoteResult{}, err
	}
	if len(mapped.Message) == 0 {
		return DeliveryNoteResult{}, fmt.Errorf("delivery note return mapping returned empty document")
	}
	if returnedQty > 0 {
		if err := applyPartialDeliveryReturnQty(mapped.Message, returnedQty); err != nil {
			return DeliveryNoteResult{}, err
		}
	}

	insertEndpoint := normalized + "/api/method/frappe.client.insert"
	var inserted struct {
		Message map[string]interface{} `json:"message"`
	}
	if err := c.doJSONRequest(
		ctx,
		http.MethodPost,
		insertEndpoint,
		apiKey,
		apiSecret,
		map[string]interface{}{"doc": mapped.Message},
		&inserted,
	); err != nil {
		return DeliveryNoteResult{}, err
	}
	if len(inserted.Message) == 0 {
		return DeliveryNoteResult{}, fmt.Errorf("delivery note return insert returned empty document")
	}
	name := strings.TrimSpace(getStringValue(inserted.Message["name"]))
	if name == "" {
		return DeliveryNoteResult{}, fmt.Errorf("delivery note return insert did not return name")
	}

	submitEndpoint := normalized + "/api/method/frappe.client.submit"
	if err := c.doJSONRequest(
		ctx,
		http.MethodPost,
		submitEndpoint,
		apiKey,
		apiSecret,
		map[string]interface{}{"doc": inserted.Message},
		nil,
	); err != nil {
		return DeliveryNoteResult{}, err
	}

	return DeliveryNoteResult{Name: name}, nil
}

func applyPartialDeliveryReturnQty(doc map[string]interface{}, returnedQty float64) error {
	items, ok := doc["items"].([]interface{})
	if !ok || len(items) == 0 {
		return fmt.Errorf("delivery note return document has no items")
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("delivery note return item has invalid shape")
	}
	firstItem["qty"] = -returnedQty
	items[0] = firstItem
	doc["items"] = items
	return nil
}

func (c *Client) EnsureDeliveryNoteStateFields(ctx context.Context, baseURL, apiKey, apiSecret string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	required := []struct {
		fieldname   string
		label       string
		fieldtype   string
		insertAfter string
		options     string
		hidden      int
	}{
		{"accord_flow_state", "Accord Flow State", "Int", "remarks", "", 1},
		{"accord_customer_state", "Accord Customer State", "Int", "accord_flow_state", "", 1},
		{"accord_customer_reason", "Accord Customer Reason", "Small Text", "accord_customer_state", "", 1},
		{"accord_delivery_actor", "Accord Delivery Actor", "Data", "accord_customer_reason", "", 1},
		{"accord_status_section", "Accord Status", "Section Break", "posting_time", "", 0},
		{"accord_ui_status", "Accord UI Status", "Select", "accord_status_section", "pending\nconfirm\npartial\nrejected", 0},
	}
	filtersJSON, _ := json.Marshal([][]interface{}{
		{"dt", "=", "Delivery Note"},
		{"fieldname", "in", []string{
			"accord_flow_state",
			"accord_customer_state",
			"accord_customer_reason",
			"accord_delivery_actor",
			"accord_status_section",
			"accord_ui_status",
		}},
	})
	params := url.Values{}
	params.Set("fields", `["name","fieldname"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", "20")
	var payload struct {
		Data []struct {
			Name          string `json:"name"`
			Fieldname     string `json:"fieldname"`
			Label         string `json:"label"`
			Fieldtype     string `json:"fieldtype"`
			InsertAfter   string `json:"insert_after"`
			Hidden        int    `json:"hidden"`
			ReadOnly      int    `json:"read_only"`
			AllowOnSubmit int    `json:"allow_on_submit"`
			NoCopy        int    `json:"no_copy"`
			Options       string `json:"options"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Custom%20Field?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return err
	}
	existing := map[string]struct {
		name          string
		label         string
		fieldtype     string
		insertAfter   string
		hidden        int
		readOnly      int
		allowOnSubmit int
		noCopy        int
		options       string
	}{}
	for _, row := range payload.Data {
		existing[strings.TrimSpace(row.Fieldname)] = struct {
			name          string
			label         string
			fieldtype     string
			insertAfter   string
			hidden        int
			readOnly      int
			allowOnSubmit int
			noCopy        int
			options       string
		}{
			name:          strings.TrimSpace(row.Name),
			label:         strings.TrimSpace(row.Label),
			fieldtype:     strings.TrimSpace(row.Fieldtype),
			insertAfter:   strings.TrimSpace(row.InsertAfter),
			hidden:        row.Hidden,
			readOnly:      row.ReadOnly,
			allowOnSubmit: row.AllowOnSubmit,
			noCopy:        row.NoCopy,
			options:       strings.TrimSpace(row.Options),
		}
	}
	for _, field := range required {
		if existingField, ok := existing[field.fieldname]; ok {
			if existingField.label == strings.TrimSpace(field.label) &&
				existingField.fieldtype == strings.TrimSpace(field.fieldtype) &&
				existingField.insertAfter == strings.TrimSpace(field.insertAfter) &&
				existingField.hidden == field.hidden &&
				existingField.readOnly == 1 &&
				existingField.allowOnSubmit == 1 &&
				existingField.noCopy == 1 &&
				existingField.options == strings.TrimSpace(field.options) {
				continue
			}
			updateEndpoint := normalized + "/api/resource/Custom%20Field/" + url.PathEscape(existingField.name)
			body := map[string]interface{}{
				"label":           field.label,
				"fieldtype":       field.fieldtype,
				"insert_after":    field.insertAfter,
				"hidden":          field.hidden,
				"read_only":       1,
				"allow_on_submit": 1,
				"no_copy":         1,
				"options":         field.options,
			}
			if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, body, nil); err != nil {
				return err
			}
			continue
		}
		createEndpoint := normalized + "/api/resource/Custom%20Field"
		body := map[string]interface{}{
			"dt":              "Delivery Note",
			"fieldname":       field.fieldname,
			"label":           field.label,
			"fieldtype":       field.fieldtype,
			"insert_after":    field.insertAfter,
			"hidden":          field.hidden,
			"read_only":       1,
			"allow_on_submit": 1,
			"no_copy":         1,
			"options":         field.options,
		}
		if err := c.doJSONRequest(ctx, http.MethodPost, createEndpoint, apiKey, apiSecret, body, nil); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return err
			}
		}
	}
	return nil
}

func (c *Client) UpdateDeliveryNoteState(ctx context.Context, baseURL, apiKey, apiSecret, name string, update DeliveryNoteStateUpdate) error {
	if err := c.EnsureDeliveryNoteStateFields(ctx, baseURL, apiKey, apiSecret); err != nil {
		return err
	}
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
		"accord_flow_state":      strings.TrimSpace(update.FlowState),
		"accord_customer_state":  strings.TrimSpace(update.CustomerState),
		"accord_customer_reason": strings.TrimSpace(update.CustomerReason),
		"accord_delivery_actor":  strings.TrimSpace(update.DeliveryActor),
		"accord_ui_status":       strings.TrimSpace(update.UIStatus),
	}, nil)
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

func (c *Client) DeleteDeliveryNote(ctx context.Context, baseURL, apiKey, apiSecret, name string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Delivery%20Note/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSONRequest(ctx, http.MethodDelete, endpoint, apiKey, apiSecret, nil, nil)
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
	// Keep list queries on scalar fields only. Frappe get_list rejects
	// rich/table fields such as remarks/items with HTTP 417.
	params.Set("fields", `["name","customer","customer_name","posting_date","modified","status","docstatus","accord_flow_state","accord_customer_state","accord_delivery_actor"]`)
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
		Name:                 getStringValue(doc["name"]),
		Customer:             getStringValue(doc["customer"]),
		CustomerName:         getStringValue(doc["customer_name"]),
		PostingDate:          getStringValue(doc["posting_date"]),
		Modified:             getStringValue(doc["modified"]),
		Status:               getStringValue(doc["status"]),
		DocStatus:            int(getFloatValue(doc["docstatus"])),
		Remarks:              getStringValue(doc["remarks"]),
		AccordFlowState:      getStringValue(doc["accord_flow_state"]),
		AccordCustomerState:  getStringValue(doc["accord_customer_state"]),
		AccordCustomerReason: getStringValue(doc["accord_customer_reason"]),
		AccordDeliveryActor:  getStringValue(doc["accord_delivery_actor"]),
		AccordUIStatus:       getStringValue(doc["accord_ui_status"]),
	}
	items, _ := doc["items"].([]interface{})
	if len(items) == 0 {
		return result, nil
	}
	firstItem, _ := items[0].(map[string]interface{})
	result.ItemCode = getStringValue(firstItem["item_code"])
	result.ItemName = getStringValue(firstItem["item_name"])
	result.Qty = getFloatValue(firstItem["qty"])
	result.ReturnedQty = getFloatValue(firstItem["returned_qty"])
	result.UOM = getStringValue(firstItem["uom"])
	if result.UOM == "" {
		result.UOM = getStringValue(firstItem["stock_uom"])
	}
	return result, nil
}

func UpsertCustomerDecisionInRemarks(existingNote, state, reason string) string {
	return UpsertCustomerDecisionPayloadInRemarks(existingNote, state, reason, 0, 0, "", "")
}

func UpsertCustomerDecisionPayloadInRemarks(existingNote, state, reason string, acceptedQty, returnedQty float64, uom, comment string) string {
	lines := strings.Split(strings.ReplaceAll(existingNote, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+5)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, compactCustomerDecisionPrefix) ||
			strings.HasPrefix(trimmed, compactCustomerReasonPrefix) ||
			strings.HasPrefix(trimmed, compactCustomerAcceptedPrefix) ||
			strings.HasPrefix(trimmed, compactCustomerReturnedPrefix) ||
			strings.HasPrefix(trimmed, compactCustomerCommentPrefix) {
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
	if acceptedQty > 0 {
		filtered = append(filtered, fmt.Sprintf("%s%.4f %s", compactCustomerAcceptedPrefix, acceptedQty, strings.TrimSpace(uom)))
	}
	if returnedQty > 0 {
		filtered = append(filtered, fmt.Sprintf("%s%.4f %s", compactCustomerReturnedPrefix, returnedQty, strings.TrimSpace(uom)))
	}
	if strings.TrimSpace(comment) != "" {
		filtered = append(filtered, compactCustomerCommentPrefix+strings.TrimSpace(comment))
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

func ExtractCustomerDecisionComment(remarks string) string {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, compactCustomerCommentPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, compactCustomerCommentPrefix))
		}
	}
	return ""
}

func ExtractCustomerDecisionQuantities(remarks string) (acceptedQty, returnedQty float64) {
	lines := strings.Split(strings.ReplaceAll(remarks, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, compactCustomerAcceptedPrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, compactCustomerAcceptedPrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				acceptedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		case strings.HasPrefix(trimmed, compactCustomerReturnedPrefix):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, compactCustomerReturnedPrefix))
			fields := strings.Fields(value)
			if len(fields) > 0 {
				returnedQty, _ = strconv.ParseFloat(fields[0], 64)
			}
		}
	}
	return acceptedQty, returnedQty
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
	case "partial", "pt":
		return "partial"
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

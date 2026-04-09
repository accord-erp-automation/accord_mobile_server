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
)

func (c *Client) SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]Customer, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
	})

	params := url.Values{}
	params.Set("fields", `["name","customer_name","mobile_no","customer_details"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", strconv.Itoa(limit))
	params.Set("order_by", "modified desc")

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"customer_name", "like", like},
			{"mobile_no", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	var payload struct {
		Data []struct {
			Name         string `json:"name"`
			CustomerName string `json:"customer_name"`
			MobileNo     string `json:"mobile_no"`
			Details      string `json:"customer_details"`
		} `json:"data"`
	}

	endpoint := normalized + "/api/resource/Customer?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Customer, 0, len(payload.Data))
	for _, row := range payload.Data {
		name := strings.TrimSpace(row.CustomerName)
		if name == "" {
			name = strings.TrimSpace(row.Name)
		}
		phone := strings.TrimSpace(row.MobileNo)
		if phone == "" {
			phone = extractPhoneFromCustomerDetails(row.Details)
		}
		items = append(items, Customer{
			ID:      strings.TrimSpace(row.Name),
			Name:    name,
			Phone:   phone,
			Details: strings.TrimSpace(row.Details),
		})
	}
	return items, nil
}

func (c *Client) GetCustomer(ctx context.Context, baseURL, apiKey, apiSecret, id string) (Customer, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Customer{}, err
	}

	endpoint := normalized + "/api/resource/Customer/" + url.PathEscape(strings.TrimSpace(id))
	var payload struct {
		Data struct {
			Name         string `json:"name"`
			CustomerName string `json:"customer_name"`
			MobileNo     string `json:"mobile_no"`
			Details      string `json:"customer_details"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return Customer{}, err
	}
	name := strings.TrimSpace(payload.Data.CustomerName)
	if name == "" {
		name = strings.TrimSpace(payload.Data.Name)
	}
	phone := strings.TrimSpace(payload.Data.MobileNo)
	if phone == "" {
		phone = extractPhoneFromCustomerDetails(payload.Data.Details)
	}
	return Customer{
		ID:      strings.TrimSpace(payload.Data.Name),
		Name:    name,
		Phone:   phone,
		Details: strings.TrimSpace(payload.Data.Details),
	}, nil
}

func (c *Client) EnsureCustomer(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateCustomerInput) (Customer, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Customer{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Customer{}, fmt.Errorf("customer name is required")
	}
	phone := strings.TrimSpace(input.Phone)

	existing, err := c.SearchCustomers(ctx, normalized, apiKey, apiSecret, name, 20)
	if err != nil {
		return Customer{}, err
	}
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return Customer{}, fmt.Errorf("ERPNext'da shu nomdagi customer allaqachon mavjud")
		}
	}

	payload := map[string]interface{}{
		"customer_name": name,
		"customer_type": "Company",
		"mobile_no":     phone,
	}

	var response struct {
		Data struct {
			Name         string `json:"name"`
			CustomerName string `json:"customer_name"`
			MobileNo     string `json:"mobile_no"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Customer"
	if err := c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, payload, &response); err != nil {
		return Customer{}, err
	}

	return Customer{
		ID:    strings.TrimSpace(response.Data.Name),
		Name:  strings.TrimSpace(response.Data.CustomerName),
		Phone: strings.TrimSpace(response.Data.MobileNo),
	}, nil
}

func (c *Client) UpdateCustomerDetails(ctx context.Context, baseURL, apiKey, apiSecret, id, details string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	endpoint := normalized + "/api/resource/Customer/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]string{
		"customer_details": strings.TrimSpace(details),
	}, nil)
}

func (c *Client) UpdateCustomerContact(ctx context.Context, baseURL, apiKey, apiSecret, id, phone, details string) error {
	return c.UpdateCustomerDetails(ctx, baseURL, apiKey, apiSecret, id, details)
}

func (c *Client) ListCustomerItems(ctx context.Context, baseURL, apiKey, apiSecret, customerRef, query string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	customer, err := c.GetCustomer(ctx, normalized, apiKey, apiSecret, customerRef)
	if err != nil {
		return nil, err
	}
	customerKeys := map[string]struct{}{}
	if trimmed := strings.ToLower(strings.TrimSpace(customer.ID)); trimmed != "" {
		customerKeys[trimmed] = struct{}{}
	}
	if trimmed := strings.ToLower(strings.TrimSpace(customer.Name)); trimmed != "" {
		customerKeys[trimmed] = struct{}{}
	}

	searchQuery := strings.TrimSpace(query)
	candidates, err := c.SearchItems(ctx, normalized, apiKey, apiSecret, searchQuery, 500)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		match, detailed, err := c.itemMatchesCustomer(ctx, normalized, apiKey, apiSecret, candidate.Code, customerKeys)
		if err != nil {
			return nil, err
		}
		if !match {
			continue
		}
		code := strings.TrimSpace(detailed.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		items = append(items, detailed)
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
	}

	sort.Slice(items, func(i, j int) bool {
		if strings.TrimSpace(query) != "" {
			leftScore := SearchQueryScore(query, items[i].Code, items[i].Name)
			rightScore := SearchQueryScore(query, items[j].Code, items[j].Name)
			if leftScore != rightScore {
				return leftScore > rightScore
			}
		}
		leftName := strings.ToLower(strings.TrimSpace(items[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(items[j].Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToLower(strings.TrimSpace(items[i].Code)) < strings.ToLower(strings.TrimSpace(items[j].Code))
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *Client) AssignCustomerToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	customer, err := c.GetCustomer(ctx, normalized, apiKey, apiSecret, customerRef)
	if err != nil {
		return err
	}
	customerLink := strings.TrimSpace(customer.ID)
	if customerLink == "" {
		return fmt.Errorf("customer not found")
	}

	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	var payload struct {
		Data struct {
			CustomerItems []struct {
				CustomerName string `json:"customer_name"`
			} `json:"customer_items"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return err
	}
	for _, row := range payload.Data.CustomerItems {
		if strings.EqualFold(strings.TrimSpace(row.CustomerName), customerLink) {
			return nil
		}
	}

	createEndpoint := normalized + "/api/resource/Item%20Customer%20Detail"
	return c.doJSONRequest(ctx, http.MethodPost, createEndpoint, apiKey, apiSecret, map[string]interface{}{
		"parent":        strings.TrimSpace(itemCode),
		"parenttype":    "Item",
		"parentfield":   "customer_items",
		"customer_name": customerLink,
		"ref_code":      customerLink,
	}, nil)
}

func (c *Client) RemoveCustomerFromItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	customer, err := c.GetCustomer(ctx, normalized, apiKey, apiSecret, customerRef)
	if err != nil {
		return err
	}
	customerLink := strings.TrimSpace(customer.ID)
	if customerLink == "" {
		return fmt.Errorf("customer not found")
	}
	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	var payload struct {
		Data struct {
			CustomerItems []struct {
				Name          string `json:"name"`
				CustomerName  string `json:"customer_name"`
				CustomerGroup string `json:"customer_group"`
				RefCode       string `json:"ref_code"`
			} `json:"customer_items"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return err
	}

	filteredRows := make([]map[string]interface{}, 0, len(payload.Data.CustomerItems))
	for _, row := range payload.Data.CustomerItems {
		if strings.EqualFold(strings.TrimSpace(row.CustomerName), customerLink) {
			continue
		}
		filteredRows = append(filteredRows, map[string]interface{}{
			"doctype":        "Item Customer Detail",
			"name":           strings.TrimSpace(row.Name),
			"customer_name":  strings.TrimSpace(row.CustomerName),
			"customer_group": strings.TrimSpace(row.CustomerGroup),
			"ref_code":       strings.TrimSpace(row.RefCode),
		})
	}

	return c.doJSONRequest(ctx, http.MethodPut, endpoint, apiKey, apiSecret, map[string]interface{}{
		"customer_items": filteredRows,
	}, nil)
}

func (c *Client) GetItemCustomerAssignment(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (ItemCustomerAssignment, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return ItemCustomerAssignment{}, err
	}
	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	var payload struct {
		Data struct {
			ItemCode      string `json:"item_code"`
			ItemName      string `json:"item_name"`
			StockUOM      string `json:"stock_uom"`
			CustomerItems []struct {
				CustomerName string `json:"customer_name"`
			} `json:"customer_items"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return ItemCustomerAssignment{}, err
	}
	refs := make([]string, 0, len(payload.Data.CustomerItems))
	seen := make(map[string]struct{}, len(payload.Data.CustomerItems))
	for _, row := range payload.Data.CustomerItems {
		ref := strings.TrimSpace(row.CustomerName)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return ItemCustomerAssignment{
		Code:         strings.TrimSpace(payload.Data.ItemCode),
		Name:         strings.TrimSpace(payload.Data.ItemName),
		UOM:          strings.TrimSpace(payload.Data.StockUOM),
		CustomerRefs: refs,
	}, nil
}

func (c *Client) itemMatchesCustomer(ctx context.Context, normalized, apiKey, apiSecret, itemCode string, customerKeys map[string]struct{}) (bool, Item, error) {
	endpoint := normalized + "/api/resource/Item/" + url.PathEscape(strings.TrimSpace(itemCode))
	var payload struct {
		Data struct {
			ItemCode      string `json:"item_code"`
			ItemName      string `json:"item_name"`
			StockUOM      string `json:"stock_uom"`
			CustomerItems []struct {
				CustomerName string `json:"customer_name"`
			} `json:"customer_items"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return false, Item{}, err
	}
	for _, row := range payload.Data.CustomerItems {
		key := strings.ToLower(strings.TrimSpace(row.CustomerName))
		if key == "" {
			continue
		}
		if _, ok := customerKeys[key]; ok {
			return true, Item{
				Code: strings.TrimSpace(payload.Data.ItemCode),
				Name: strings.TrimSpace(payload.Data.ItemName),
				UOM:  strings.TrimSpace(payload.Data.StockUOM),
			}, nil
		}
	}
	return false, Item{}, nil
}

func extractPhoneFromCustomerDetails(details string) string {
	lines := strings.Split(strings.ReplaceAll(details, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "telefon:") {
			return strings.TrimSpace(trimmed[len("telefon:"):])
		}
		if strings.HasPrefix(lower, "phone:") {
			return strings.TrimSpace(trimmed[len("phone:"):])
		}
	}
	return ""
}

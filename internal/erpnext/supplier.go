package erpnext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"mobile_server/internal/suplier"
)

type CreateSupplierInput struct {
	Name  string
	Phone string
}

func (c *Client) SearchSuppliers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]Supplier, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	filtersJSON, _ := json.Marshal([][]interface{}{
		{"disabled", "=", 0},
	})

	params := url.Values{}
	params.Set("fields", `["name","supplier_name","mobile_no"]`)
	params.Set("filters", string(filtersJSON))
	params.Set("limit_page_length", strconv.Itoa(limit))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		like := "%" + strings.ReplaceAll(trimmed, "\"", "") + "%"
		orFiltersJSON, _ := json.Marshal([][]interface{}{
			{"name", "like", like},
			{"supplier_name", "like", like},
			{"mobile_no", "like", like},
		})
		params.Set("or_filters", string(orFiltersJSON))
	}

	var payload struct {
		Data []struct {
			Name         string `json:"name"`
			SupplierName string `json:"supplier_name"`
			MobileNo     string `json:"mobile_no"`
			Details      string `json:"supplier_details"`
		} `json:"data"`
	}

	params.Set("fields", `["name","supplier_name","mobile_no","supplier_details"]`)
	endpoint := normalized + "/api/resource/Supplier?" + params.Encode()
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return nil, err
	}

	items := make([]Supplier, 0, len(payload.Data))
	for _, row := range payload.Data {
		name := strings.TrimSpace(row.SupplierName)
		if name == "" {
			name = strings.TrimSpace(row.Name)
		}
		phone := strings.TrimSpace(row.MobileNo)
		if phone == "" {
			phone = extractPhoneFromSupplierDetails(row.Details)
		}
		items = append(items, Supplier{
			ID:    strings.TrimSpace(row.Name),
			Name:  name,
			Phone: phone,
		})
	}
	return items, nil
}

func (c *Client) EnsureSupplier(ctx context.Context, baseURL, apiKey, apiSecret string, input CreateSupplierInput) (Supplier, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Supplier{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Supplier{}, fmt.Errorf("supplier name is required")
	}
	phone := strings.TrimSpace(input.Phone)

	existing, err := c.SearchSuppliers(ctx, normalized, apiKey, apiSecret, name, 20)
	if err != nil {
		return Supplier{}, err
	}
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return Supplier{}, fmt.Errorf("ERPNext'da shu nomdagi supplier allaqachon mavjud")
		}
		if phone != "" && strings.EqualFold(strings.TrimSpace(item.Phone), phone) {
			return Supplier{}, fmt.Errorf("ERPNext'da shu telefon raqam bilan supplier allaqachon mavjud")
		}
	}

	details := ""
	if phone != "" {
		details = "Telefon: " + phone
	}

	payload := map[string]interface{}{
		"supplier_name":    name,
		"supplier_type":    "Company",
		"supplier_group":   "Services",
		"mobile_no":        phone,
		"supplier_details": details,
	}

	var response struct {
		Data struct {
			Name         string `json:"name"`
			SupplierName string `json:"supplier_name"`
			MobileNo     string `json:"mobile_no"`
		} `json:"data"`
	}
	endpoint := normalized + "/api/resource/Supplier"
	if err := c.doJSONRequest(ctx, http.MethodPost, endpoint, apiKey, apiSecret, payload, &response); err != nil {
		return Supplier{}, err
	}

	return Supplier{
		ID:    strings.TrimSpace(response.Data.Name),
		Name:  strings.TrimSpace(response.Data.SupplierName),
		Phone: strings.TrimSpace(response.Data.MobileNo),
	}, nil
}

func (c *Client) GetSupplier(ctx context.Context, baseURL, apiKey, apiSecret, id string) (Supplier, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Supplier{}, err
	}

	endpoint := normalized + "/api/resource/Supplier/" + url.PathEscape(strings.TrimSpace(id))
	var payload struct {
		Data struct {
			Name         string `json:"name"`
			SupplierName string `json:"supplier_name"`
			MobileNo     string `json:"mobile_no"`
			Details      string `json:"supplier_details"`
			Image        string `json:"image"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, endpoint, apiKey, apiSecret, &payload); err != nil {
		return Supplier{}, err
	}

	name := strings.TrimSpace(payload.Data.SupplierName)
	if name == "" {
		name = strings.TrimSpace(payload.Data.Name)
	}
	phone := strings.TrimSpace(payload.Data.MobileNo)
	if phone == "" {
		phone = extractPhoneFromSupplierDetails(payload.Data.Details)
	}

	return Supplier{
		ID:    strings.TrimSpace(payload.Data.Name),
		Name:  name,
		Phone: phone,
		Image: strings.TrimSpace(payload.Data.Image),
	}, nil
}

func (c *Client) UploadSupplierImage(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(supplierID) == "" {
		return "", fmt.Errorf("supplier id is required")
	}
	if len(content) == 0 {
		return "", fmt.Errorf("image content is required")
	}

	fileURL, err := c.uploadFile(ctx, normalized, apiKey, apiSecret, supplierID, filename, contentType, content)
	if err != nil {
		return "", err
	}

	updateEndpoint := normalized + "/api/resource/Supplier/" + url.PathEscape(strings.TrimSpace(supplierID))
	if err := c.doJSONRequest(ctx, http.MethodPut, updateEndpoint, apiKey, apiSecret, map[string]string{
		"image": fileURL,
	}, nil); err != nil {
		return "", err
	}

	return fileURL, nil
}

func (c *Client) uploadFile(ctx context.Context, baseURL, apiKey, apiSecret, supplierID, filename, contentType string, content []byte) (string, error) {
	if strings.TrimSpace(filename) == "" {
		filename = "avatar.png"
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("doctype", "Supplier"); err != nil {
		return "", err
	}
	if err := writer.WriteField("docname", strings.TrimSpace(supplierID)); err != nil {
		return "", err
	}
	if err := writer.WriteField("is_private", "0"); err != nil {
		return "", err
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(content); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/method/upload_file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", apiKey, apiSecret))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Message struct {
			FileURL string `json:"file_url"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Message.FileURL) == "" {
		return "", fmt.Errorf("upload_file did not return file_url")
	}
	return strings.TrimSpace(payload.Message.FileURL), nil
}

func extractPhoneFromSupplierDetails(details string) string {
	lines := strings.Split(details, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if idx := strings.Index(trimmed, ":"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[idx+1:])
		}
		if phone, err := suplier.NormalizePhone(trimmed); err == nil {
			return phone
		}
	}
	return ""
}

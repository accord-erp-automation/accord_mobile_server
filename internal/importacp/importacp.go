package importacp

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"mobile_server/internal/erpnext"
)

type ERP interface {
	SearchCustomers(ctx context.Context, baseURL, apiKey, apiSecret, query string, limit int) ([]erpnext.Customer, error)
	EnsureCustomer(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateCustomerInput) (erpnext.Customer, error)
	GetItemsByCodes(ctx context.Context, baseURL, apiKey, apiSecret string, itemCodes []string) ([]erpnext.Item, error)
	GetItemCustomerAssignment(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string) (erpnext.ItemCustomerAssignment, error)
	CreateItem(ctx context.Context, baseURL, apiKey, apiSecret string, input erpnext.CreateItemInput) (erpnext.Item, error)
	AssignCustomerToItem(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, customerRef string) error
	UpdateItemStandardRate(ctx context.Context, baseURL, apiKey, apiSecret, itemCode string, rate float64) error
	UpsertItemBarcode(ctx context.Context, baseURL, apiKey, apiSecret, itemCode, barcode, uom string) error
}

type Row struct {
	Agent   string
	Name    string
	Price   float64
	Barcode string
}

type Options struct {
	CSVPath   string
	UOM       string
	ItemGroup string
	BaseURL   string
	APIKey    string
	APISecret string
	DryRun    bool
}

type Result struct {
	RowsRead         int
	ItemsFound       int
	CustomersCreated []string
	ItemsCreated     []string
	ItemsExisting    []string
	Assignments      []string
	PricesUpdated    []string
	BarcodesUpdated  []string
}

type ImportEntry struct {
	Agent   string
	Name    string
	Price   float64
	Barcode string
}

func Run(ctx context.Context, erp ERP, out io.Writer, opts Options) (Result, error) {
	if strings.TrimSpace(opts.CSVPath) == "" {
		return Result{}, fmt.Errorf("csv path is required")
	}
	if strings.TrimSpace(opts.BaseURL) == "" || strings.TrimSpace(opts.APIKey) == "" || strings.TrimSpace(opts.APISecret) == "" {
		return Result{}, fmt.Errorf("erp credentials are required")
	}
	if strings.TrimSpace(opts.UOM) == "" {
		opts.UOM = "Kg"
	}
	if strings.TrimSpace(opts.ItemGroup) == "" {
		opts.ItemGroup = "Tayyor mahsulot"
	}

	rows, err := loadRows(opts.CSVPath)
	if err != nil {
		return Result{}, err
	}
	entries, rowsRead, err := normalizeRows(rows)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		RowsRead:   rowsRead,
		ItemsFound: len(entries),
	}

	customersByAgent, createdCustomers, err := ensureCustomers(ctx, erp, opts, entries)
	if err != nil {
		return result, err
	}
	result.CustomersCreated = createdCustomers

	existingItems, err := resolveExistingItems(ctx, erp, opts, entries)
	if err != nil {
		return result, err
	}

	if err := preflightConflicts(ctx, erp, opts, entries, customersByAgent, existingItems); err != nil {
		return result, err
	}

	for _, row := range entries {
		customer := customersByAgent[row.Agent]
		itemCode := row.Name
		if existing, ok := existingItems[strings.ToLower(strings.TrimSpace(row.Name))]; ok &&
			strings.EqualFold(strings.TrimSpace(existing.Code), itemCode) {
			result.ItemsExisting = append(result.ItemsExisting, itemCode)
		} else {
			if !opts.DryRun {
				created, err := erp.CreateItem(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, erpnext.CreateItemInput{
					Code:      itemCode,
					Name:      row.Name,
					UOM:       opts.UOM,
					ItemGroup: opts.ItemGroup,
				})
				if err != nil {
					return result, fmt.Errorf("create item %q: %w", row.Name, err)
				}
				itemCode = created.Code
			}
			result.ItemsCreated = append(result.ItemsCreated, itemCode)
		}

		if !opts.DryRun {
			if err := erp.AssignCustomerToItem(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, itemCode, customer.ID); err != nil {
				return result, fmt.Errorf("assign customer %q to item %q: %w", customer.ID, itemCode, err)
			}
			if err := erp.UpdateItemStandardRate(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, itemCode, row.Price); err != nil {
				return result, fmt.Errorf("update standard_rate for %q: %w", itemCode, err)
			}
			if err := erp.UpsertItemBarcode(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, itemCode, row.Barcode, opts.UOM); err != nil {
				return result, fmt.Errorf("upsert barcode for %q: %w", itemCode, err)
			}
		}
		result.Assignments = append(result.Assignments, itemCode)
		result.PricesUpdated = append(result.PricesUpdated, itemCode)
		if strings.TrimSpace(row.Barcode) != "" {
			result.BarcodesUpdated = append(result.BarcodesUpdated, itemCode)
		}
	}

	if out != nil {
		fmt.Fprintf(out, "rows read: %d\n", result.RowsRead)
		fmt.Fprintf(out, "items found: %d\n", result.ItemsFound)
		fmt.Fprintf(out, "customers created: %d\n", len(result.CustomersCreated))
		fmt.Fprintf(out, "items created: %d\n", len(result.ItemsCreated))
		fmt.Fprintf(out, "items existing: %d\n", len(result.ItemsExisting))
		fmt.Fprintf(out, "assignments: %d\n", len(result.Assignments))
		fmt.Fprintf(out, "prices updated: %d\n", len(result.PricesUpdated))
		fmt.Fprintf(out, "barcodes updated: %d\n", len(result.BarcodesUpdated))
	}
	return result, nil
}

func resolveExistingItems(ctx context.Context, erp ERP, opts Options, rows []ImportEntry) (map[string]erpnext.Item, error) {
	keys := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		code := strings.TrimSpace(row.Name)
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, code)
	}

	result := map[string]erpnext.Item{}
	for start := 0; start < len(keys); start += 100 {
		end := start + 100
		if end > len(keys) {
			end = len(keys)
		}
		items, err := erp.GetItemsByCodes(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, keys[start:end])
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item.Code))
			if key == "" {
				continue
			}
			result[key] = item
		}
	}
	return result, nil
}

func normalizeRows(rows []Row) ([]ImportEntry, int, error) {
	rowsRead := 0
	entriesByName := map[string]ImportEntry{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Agent) == "" || strings.TrimSpace(row.Name) == "" {
			continue
		}
		rowsRead++
		key := strings.ToLower(strings.TrimSpace(row.Name))
		if existing, ok := entriesByName[key]; ok {
			if !strings.EqualFold(strings.TrimSpace(existing.Agent), strings.TrimSpace(row.Agent)) {
				return nil, rowsRead, fmt.Errorf("csv item %q mapped to multiple agents: %q and %q", row.Name, existing.Agent, row.Agent)
			}
			if existing.Price == 0 && row.Price != 0 {
				existing.Price = row.Price
			}
			if existing.Barcode == "" && strings.TrimSpace(row.Barcode) != "" {
				existing.Barcode = strings.TrimSpace(row.Barcode)
			}
			entriesByName[key] = existing
			continue
		}
		entriesByName[key] = ImportEntry{
			Agent:   strings.TrimSpace(row.Agent),
			Name:    strings.TrimSpace(row.Name),
			Price:   row.Price,
			Barcode: strings.TrimSpace(row.Barcode),
		}
		order = append(order, key)
	}
	if len(entriesByName) == 0 {
		return nil, rowsRead, fmt.Errorf("csv produced no rows")
	}
	result := make([]ImportEntry, 0, len(entriesByName))
	for _, key := range order {
		result = append(result, entriesByName[key])
	}
	return result, rowsRead, nil
}

func loadRows(path string) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("csv is empty")
	}
	header := rows[0]
	if len(header) < 4 {
		return nil, fmt.Errorf("csv must have Agent,Nom,Price,Barcode columns")
	}

	result := make([]Row, 0, len(rows)-1)
	start := 0
	if len(rows) > 0 {
		first := rows[0]
		if len(first) >= 4 &&
			strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(first[0], "\ufeff")), "Agent") &&
			strings.EqualFold(strings.TrimSpace(first[1]), "Nom") {
			start = 1
		}
	}
	for _, row := range rows[start:] {
		if len(row) < 4 {
			continue
		}
		agent := strings.TrimSpace(row[0])
		name := strings.TrimSpace(row[1])
		priceRaw := strings.TrimSpace(row[2])
		barcode := strings.TrimSpace(row[3])
		if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(agent, "\ufeff")), "Agent") &&
			strings.EqualFold(name, "Nom") &&
			strings.EqualFold(priceRaw, "Price") &&
			strings.EqualFold(barcode, "Barcode") {
			continue
		}
		if agent == "" || name == "" {
			continue
		}
		price, err := parseACPPrice(priceRaw)
		if err != nil {
			return nil, fmt.Errorf("parse price for %q: %w", name, err)
		}
		result = append(result, Row{
			Agent:   agent,
			Name:    name,
			Price:   price,
			Barcode: barcode,
		})
	}
	return result, nil
}

func parseACPPrice(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	normalized := strings.ReplaceAll(trimmed, ",", ".")
	return strconv.ParseFloat(normalized, 64)
}

func ensureCustomers(ctx context.Context, erp ERP, opts Options, rows []ImportEntry) (map[string]erpnext.Customer, []string, error) {
	existingCustomers, err := erp.SearchCustomers(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, "", 1000)
	if err != nil {
		return nil, nil, err
	}
	customerByKey := map[string]erpnext.Customer{}
	for _, customer := range existingCustomers {
		if trimmed := strings.TrimSpace(customer.ID); trimmed != "" {
			customerByKey[strings.ToLower(trimmed)] = customer
		}
		if trimmed := strings.TrimSpace(customer.Name); trimmed != "" {
			customerByKey[strings.ToLower(trimmed)] = customer
		}
	}
	byAgent := map[string]erpnext.Customer{}
	created := []string{}
	for _, row := range rows {
		if _, ok := byAgent[row.Agent]; ok {
			continue
		}
		query := strings.TrimSpace(row.Agent)
		if exact, ok := customerByKey[strings.ToLower(query)]; ok {
			byAgent[row.Agent] = exact
			continue
		}
		if opts.DryRun {
			byAgent[row.Agent] = erpnext.Customer{ID: query, Name: query}
			created = append(created, query)
			continue
		}
		createdCustomer, err := erp.EnsureCustomer(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, erpnext.CreateCustomerInput{
			Name: query,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("ensure customer %q: %w", query, err)
		}
		customerByKey[strings.ToLower(createdCustomer.ID)] = createdCustomer
		customerByKey[strings.ToLower(createdCustomer.Name)] = createdCustomer
		byAgent[row.Agent] = createdCustomer
		created = append(created, createdCustomer.ID)
	}
	return byAgent, created, nil
}

func preflightConflicts(ctx context.Context, erp ERP, opts Options, rows []ImportEntry, customers map[string]erpnext.Customer, existingItems map[string]erpnext.Item) error {
	for _, row := range rows {
		if _, ok := existingItems[strings.ToLower(strings.TrimSpace(row.Name))]; !ok {
			continue
		}
		target := strings.TrimSpace(customers[row.Agent].ID)
		assignment, err := erp.GetItemCustomerAssignment(ctx, opts.BaseURL, opts.APIKey, opts.APISecret, row.Name)
		if err != nil {
			return err
		}
		conflicts := make([]string, 0, len(assignment.CustomerRefs))
		for _, ref := range assignment.CustomerRefs {
			trimmed := strings.TrimSpace(ref)
			if trimmed == "" || strings.EqualFold(trimmed, target) {
				continue
			}
			conflicts = append(conflicts, trimmed)
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("item %q already linked to customer(s): %s", row.Name, strings.Join(conflicts, ", "))
		}
	}
	return nil
}

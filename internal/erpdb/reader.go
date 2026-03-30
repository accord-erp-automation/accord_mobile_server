package erpdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"mobile_server/internal/core"
	"mobile_server/internal/erpnext"
)

type Config struct {
	Host             string
	Port             int
	Name             string
	User             string
	Password         string
	DefaultWarehouse string
	MaxOpenConns     int
	MaxIdleConns     int
	MaxIdleTime      time.Duration
}

type siteConfig struct {
	DBName     string `json:"db_name"`
	DBPassword string `json:"db_password"`
	DBType     string `json:"db_type"`
}

type Reader struct {
	db               *sql.DB
	defaultWarehouse string
}

type purchaseReceiptSummaryRow struct {
	Supplier             string
	DocStatus            int
	Status               string
	TotalQty             float64
	SupplierDeliveryNote string
	Remarks              string
}

type deliveryNoteSummaryRow struct {
	Customer            string
	DocStatus           int
	Qty                 float64
	ReturnedQty         float64
	AccordFlowState     int
	AccordCustomerState int
}

const (
	deliveryFlowStateSubmittedDB = 1
	customerStateRejectedDB      = 2
	customerStateConfirmedDB     = 3
	customerStatePartialDB       = 4
)

func ConfigFromSiteConfig(siteConfigPath, defaultWarehouse string) (Config, error) {
	raw, err := os.ReadFile(strings.TrimSpace(siteConfigPath))
	if err != nil {
		return Config{}, err
	}
	var cfg siteConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.DBType) != "" && !strings.EqualFold(strings.TrimSpace(cfg.DBType), "mariadb") {
		return Config{}, fmt.Errorf("unsupported db_type %q", cfg.DBType)
	}
	name := strings.TrimSpace(cfg.DBName)
	if name == "" {
		return Config{}, fmt.Errorf("db_name is required")
	}
	return Config{
		Host:             "127.0.0.1",
		Port:             3306,
		Name:             name,
		User:             name,
		Password:         strings.TrimSpace(cfg.DBPassword),
		DefaultWarehouse: strings.TrimSpace(defaultWarehouse),
		MaxOpenConns:     12,
		MaxIdleConns:     12,
		MaxIdleTime:      5 * time.Minute,
	}, nil
}

func Open(cfg Config) (*Reader, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port <= 0 {
		port = 3306
	}
	user := strings.TrimSpace(cfg.User)
	if user == "" {
		user = strings.TrimSpace(cfg.Name)
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, fmt.Errorf("db name is required")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC&collation=utf8mb4_unicode_ci",
		user,
		cfg.Password,
		host,
		port,
		name,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.MaxIdleTime)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Reader{
		db:               db,
		defaultWarehouse: strings.TrimSpace(cfg.DefaultWarehouse),
	}, nil
}

func ParsePort(raw string, fallback int) int {
	if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func (r *Reader) SearchWerkaSuppliersPage(ctx context.Context, query string, limit, offset int) ([]core.SupplierDirectoryEntry, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			s.name,
			COALESCE(NULLIF(s.supplier_name, ''), s.name) AS supplier_name,
			COALESCE(s.mobile_no, '')
		FROM `+"`tabItem Supplier`"+` isup
		INNER JOIN tabSupplier s ON s.name = isup.supplier
		INNER JOIN tabItem i ON i.name = isup.parent
		WHERE s.disabled = 0
		  AND i.disabled = 0
		  AND (? = '' OR s.name LIKE ? ESCAPE '\\' OR s.supplier_name LIKE ? ESCAPE '\\' OR COALESCE(s.mobile_no, '') LIKE ? ESCAPE '\\')
		ORDER BY s.modified DESC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query), like, like, like, limit, max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.SupplierDirectoryEntry, 0, limit)
	for rows.Next() {
		var item core.SupplierDirectoryEntry
		if err := rows.Scan(&item.Ref, &item.Name, &item.Phone); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Reader) SearchWerkaCustomersPage(ctx context.Context, query string, limit, offset int) ([]core.CustomerDirectoryEntry, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			c.name,
			COALESCE(NULLIF(c.customer_name, ''), c.name) AS customer_name,
			COALESCE(c.mobile_no, '')
		FROM tabCustomer c
		INNER JOIN `+"`tabItem Customer Detail`"+` icd ON icd.customer_name = c.name
		INNER JOIN tabItem i ON i.name = icd.parent
		WHERE c.disabled = 0
		  AND i.disabled = 0
		  AND (? = '' OR c.name LIKE ? ESCAPE '\\' OR c.customer_name LIKE ? ESCAPE '\\' OR COALESCE(c.mobile_no, '') LIKE ? ESCAPE '\\')
		ORDER BY c.modified DESC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query), like, like, like, limit, max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.CustomerDirectoryEntry, 0, limit)
	for rows.Next() {
		var item core.CustomerDirectoryEntry
		if err := rows.Scan(&item.Ref, &item.Name, &item.Phone); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Reader) SearchWerkaCustomerItemsPage(ctx context.Context, customerRef, query string, limit, offset int) ([]core.SupplierItem, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
		FROM `+"`tabItem Customer Detail`"+` icd
		INNER JOIN tabItem i ON i.name = icd.parent
		WHERE icd.customer_name = ?
		  AND i.disabled = 0
		  AND (? = '' OR i.item_code LIKE ? ESCAPE '\\' OR i.item_name LIKE ? ESCAPE '\\')
		ORDER BY i.item_name ASC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(customerRef),
		strings.TrimSpace(query), like, like, limit, max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.SupplierItem, 0, limit)
	for rows.Next() {
		var item core.SupplierItem
		if err := rows.Scan(&item.Code, &item.Name, &item.UOM); err != nil {
			return nil, err
		}
		item.Warehouse = r.defaultWarehouse
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Reader) SearchWerkaSupplierItemsPage(ctx context.Context, supplierRef, query string, limit, offset int) ([]core.SupplierItem, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
		FROM `+"`tabItem Supplier`"+` isup
		INNER JOIN tabItem i ON i.name = isup.parent
		WHERE isup.supplier = ?
		  AND i.disabled = 0
		  AND (? = '' OR i.item_code LIKE ? ESCAPE '\\' OR i.item_name LIKE ? ESCAPE '\\')
		ORDER BY i.item_name ASC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(supplierRef),
		strings.TrimSpace(query), like, like, limit, max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.SupplierItem, 0, limit)
	for rows.Next() {
		var item core.SupplierItem
		if err := rows.Scan(&item.Code, &item.Name, &item.UOM); err != nil {
			return nil, err
		}
		item.Warehouse = r.defaultWarehouse
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Reader) SearchWerkaCustomerItemOptionsPage(ctx context.Context, query string, limit, offset int) ([]core.CustomerItemOption, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			c.name,
			COALESCE(NULLIF(c.customer_name, ''), c.name) AS customer_name,
			COALESCE(c.mobile_no, '') AS mobile_no,
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
		FROM `+"`tabItem Customer Detail`"+` icd
		INNER JOIN tabItem i ON i.name = icd.parent
		INNER JOIN tabCustomer c ON c.name = icd.customer_name
		WHERE c.disabled = 0
		  AND i.disabled = 0
		  AND (
			? = ''
			OR i.item_code LIKE ? ESCAPE '\\'
			OR i.item_name LIKE ? ESCAPE '\\'
			OR c.name LIKE ? ESCAPE '\\'
			OR c.customer_name LIKE ? ESCAPE '\\'
			OR COALESCE(c.mobile_no, '') LIKE ? ESCAPE '\\'
		  )
		ORDER BY i.item_name ASC, c.customer_name ASC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query), like, like, like, like, like, limit, max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.CustomerItemOption, 0, limit)
	for rows.Next() {
		var item core.CustomerItemOption
		if err := rows.Scan(
			&item.CustomerRef,
			&item.CustomerName,
			&item.CustomerPhone,
			&item.ItemCode,
			&item.ItemName,
			&item.UOM,
		); err != nil {
			return nil, err
		}
		item.Warehouse = r.defaultWarehouse
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Reader) WerkaSummary(ctx context.Context) (core.WerkaHomeSummary, error) {
	receipts, err := r.telegramReceiptRows(ctx, "")
	if err != nil {
		return core.WerkaHomeSummary{}, err
	}
	summary := core.WerkaHomeSummary{}
	for _, row := range receipts {
		status, include := classifyWerkaReceipt(row)
		if !include {
			continue
		}
		switch status {
		case "pending", "draft":
			summary.PendingCount++
		case "accepted":
			summary.ConfirmedCount++
		case "partial", "rejected", "cancelled":
			summary.ReturnedCount++
		}
	}
	deliveryNotes, err := r.deliveryNoteRows(ctx, "")
	if err != nil {
		return core.WerkaHomeSummary{}, err
	}
	for _, row := range deliveryNotes {
		if !deliveryVisible(row) {
			continue
		}
		switch deliveryStatus(row) {
		case "pending":
			summary.PendingCount++
		case "accepted":
			summary.ConfirmedCount++
		case "partial", "rejected", "cancelled":
			summary.ReturnedCount++
		}
	}
	return summary, nil
}

func (r *Reader) SupplierSummary(ctx context.Context, supplierRef string) (core.SupplierHomeSummary, error) {
	rows, err := r.telegramReceiptRows(ctx, supplierRef)
	if err != nil {
		return core.SupplierHomeSummary{}, err
	}
	summary := core.SupplierHomeSummary{}
	for _, row := range rows {
		status, _ := classifyWerkaReceipt(row)
		switch status {
		case "pending", "draft":
			summary.PendingCount++
		case "accepted":
			summary.SubmittedCount++
		case "partial", "rejected", "cancelled":
			summary.ReturnedCount++
		}
	}
	return summary, nil
}

func (r *Reader) CustomerSummary(ctx context.Context, customerRef string) (core.CustomerHomeSummary, error) {
	rows, err := r.deliveryNoteRows(ctx, customerRef)
	if err != nil {
		return core.CustomerHomeSummary{}, err
	}
	summary := core.CustomerHomeSummary{}
	for _, row := range rows {
		if !deliveryVisible(row) {
			continue
		}
		switch deliveryStatus(row) {
		case "accepted":
			summary.ConfirmedCount++
		case "partial", "rejected":
			summary.RejectedCount++
		default:
			summary.PendingCount++
		}
	}
	return summary, nil
}

func (r *Reader) telegramReceiptRows(ctx context.Context, supplierRef string) ([]purchaseReceiptSummaryRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT supplier, docstatus, COALESCE(status, ''), COALESCE(total_qty, 0), COALESCE(supplier_delivery_note, ''), COALESCE(remarks, '')
		FROM `+"`tabPurchase Receipt`"+`
		WHERE supplier_delivery_note LIKE 'TG:%'
		  AND (? = '' OR supplier = ?)
	`, strings.TrimSpace(supplierRef), strings.TrimSpace(supplierRef))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]purchaseReceiptSummaryRow, 0, 64)
	for rows.Next() {
		var row purchaseReceiptSummaryRow
		if err := rows.Scan(
			&row.Supplier,
			&row.DocStatus,
			&row.Status,
			&row.TotalQty,
			&row.SupplierDeliveryNote,
			&row.Remarks,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Reader) deliveryNoteRows(ctx context.Context, customerRef string) ([]deliveryNoteSummaryRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT customer, docstatus, COALESCE(total_qty, 0), COALESCE(per_returned, 0), COALESCE(accord_flow_state, 0), COALESCE(accord_customer_state, 0)
		FROM `+"`tabDelivery Note`"+`
		WHERE (? = '' OR customer = ?)
	`, strings.TrimSpace(customerRef), strings.TrimSpace(customerRef))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]deliveryNoteSummaryRow, 0, 64)
	for rows.Next() {
		var row deliveryNoteSummaryRow
		if err := rows.Scan(
			&row.Customer,
			&row.DocStatus,
			&row.Qty,
			&row.ReturnedQty,
			&row.AccordFlowState,
			&row.AccordCustomerState,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func classifyWerkaReceipt(row purchaseReceiptSummaryRow) (string, bool) {
	sentQty := row.TotalQty
	if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(row.SupplierDeliveryNote); ok && markerQty > sentQty {
		sentQty = markerQty
	}

	status := "pending"
	switch {
	case row.DocStatus == 2 || strings.EqualFold(strings.TrimSpace(row.Status), "Cancelled"):
		status = "cancelled"
	case row.DocStatus == 1:
		status = purchaseReceiptStatusFromQuantities(sentQty, row.TotalQty)
	case strings.EqualFold(strings.TrimSpace(row.Status), "Draft"):
		status = "draft"
	}

	unannouncedState := strings.TrimSpace(erpnext.ExtractWerkaUnannouncedState(row.Remarks))
	if row.DocStatus == 0 && unannouncedState == "pending" {
		return status, false
	}
	if status == "accepted" && unannouncedState == "approved" {
		return status, false
	}
	return status, true
}

func deliveryVisible(row deliveryNoteSummaryRow) bool {
	return row.DocStatus == 1 && row.AccordFlowState == deliveryFlowStateSubmittedDB
}

func deliveryStatus(row deliveryNoteSummaryRow) string {
	if row.DocStatus != 1 {
		return "draft"
	}
	if row.AccordFlowState != deliveryFlowStateSubmittedDB {
		return "pending"
	}
	switch row.AccordCustomerState {
	case customerStateRejectedDB:
		return "rejected"
	case customerStateConfirmedDB:
		return "accepted"
	case customerStatePartialDB:
		return "partial"
	default:
		return "pending"
	}
}

func purchaseReceiptStatusFromQuantities(sentQty, acceptedQty float64) string {
	switch {
	case acceptedQty <= 0:
		return "rejected"
	case sentQty > 0 && acceptedQty < sentQty:
		return "partial"
	default:
		return "accepted"
	}
}

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if max > 0 && value > max {
		value = max
	}
	return value
}

func max(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}

func likePattern(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "%"
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(trimmed) + "%"
}

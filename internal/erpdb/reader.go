package erpdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
	Name                 string
	Supplier             string
	SupplierName         string
	DocStatus            int
	Status               string
	TotalQty             float64
	PostingDate          string
	SupplierDeliveryNote string
	Remarks              string
	Currency             string
	ItemCode             string
	ItemName             string
	UOM                  string
	Amount               float64
}

type deliveryNoteSummaryRow struct {
	Name                string
	Customer            string
	CustomerName        string
	DocStatus           int
	Modified            string
	Qty                 float64
	ReturnedQty         float64
	CustomerReason      string
	ItemCode            string
	ItemName            string
	UOM                 string
	AccordFlowState     int
	AccordCustomerState int
}

type purchaseReceiptStatusRow struct {
	DocStatus            int
	Status               string
	TotalQty             float64
	SupplierDeliveryNote string
	Remarks              string
}

type deliveryNoteStatusRow struct {
	DocStatus           int
	AccordFlowState     int
	AccordCustomerState int
}

type supplierItemSearchEntry struct {
	item        core.SupplierItem
	searchTerms []string
}

type searchPatterns struct {
	primaryQuery   string
	primaryLike    string
	secondaryQuery string
	secondaryLike  string
}

const (
	deliveryFlowStateSubmittedDB = 1
	customerStateRejectedDB      = 2
	customerStateConfirmedDB     = 3
	customerStatePartialDB       = 4
	supplierAckEventPrefixDB     = "supplier_ack:"
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
	if strings.TrimSpace(query) == "" {
		rows, err := r.db.QueryContext(ctx, `
			SELECT DISTINCT
				i.item_code,
				COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
				COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
			FROM `+"`tabItem Customer Detail`"+` icd
			INNER JOIN tabItem i ON i.name = icd.parent
			WHERE icd.customer_name = ?
			  AND i.disabled = 0
			ORDER BY i.item_name ASC
			LIMIT ? OFFSET ?`,
			strings.TrimSpace(customerRef),
			limit,
			max(offset, 0),
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

	items, err := r.loadAllWerkaCustomerItemSearchEntries(ctx, customerRef)
	if err != nil {
		return nil, err
	}
	return slicePage(rankSupplierItemSearchEntriesByQuery(items, query), offset, limit), nil
}

func (r *Reader) SearchWerkaSupplierItemsPage(ctx context.Context, supplierRef, query string, limit, offset int) ([]core.SupplierItem, error) {
	limit = clampLimit(limit, 50, 500)
	if strings.TrimSpace(query) == "" {
		rows, err := r.db.QueryContext(ctx, `
			SELECT DISTINCT
				i.item_code,
				COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
				COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
			FROM `+"`tabItem Supplier`"+` isup
			INNER JOIN tabItem i ON i.name = isup.parent
			WHERE isup.supplier = ?
			  AND i.disabled = 0
			ORDER BY i.item_name ASC
			LIMIT ? OFFSET ?`,
			strings.TrimSpace(supplierRef),
			limit,
			max(offset, 0),
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

	items, err := r.loadAllWerkaSupplierItems(ctx, supplierRef)
	if err != nil {
		return nil, err
	}
	return slicePage(rankSupplierItemsByQuery(items, query), offset, limit), nil
}

func (r *Reader) SearchWerkaCustomerItemOptionsPage(ctx context.Context, query string, limit, offset int) ([]core.CustomerItemOption, error) {
	limit = clampLimit(limit, 50, 500)
	if strings.TrimSpace(query) == "" {
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
			ORDER BY i.item_name ASC, c.customer_name ASC
			LIMIT ? OFFSET ?`,
			limit,
			max(offset, 0),
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

	items, err := r.loadAllWerkaCustomerItemOptions(ctx)
	if err != nil {
		return nil, err
	}
	return slicePage(rankCustomerItemOptionsByQuery(items, query), offset, limit), nil
}

func (r *Reader) WerkaSummary(ctx context.Context) (core.WerkaHomeSummary, error) {
	receipts, err := r.telegramReceiptStatusRows(ctx)
	if err != nil {
		return core.WerkaHomeSummary{}, err
	}
	summary := core.WerkaHomeSummary{}
	for _, row := range receipts {
		status, include := classifyWerkaReceiptStatusRow(row)
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
	deliveryNotes, err := r.deliveryNoteStatusRows(ctx)
	if err != nil {
		return core.WerkaHomeSummary{}, err
	}
	for _, row := range deliveryNotes {
		if !deliveryStatusVisible(row) {
			continue
		}
		switch deliveryStatusFromState(row.DocStatus, row.AccordFlowState, row.AccordCustomerState) {
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

func (r *Reader) telegramReceiptStatusRows(ctx context.Context) ([]purchaseReceiptStatusRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			pr.docstatus,
			COALESCE(pr.status, ''),
			COALESCE(pr.total_qty, 0),
			COALESCE(pr.supplier_delivery_note, ''),
			COALESCE(pr.remarks, '')
		FROM `+"`tabPurchase Receipt`"+` pr
		WHERE pr.supplier_delivery_note LIKE 'TG:%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]purchaseReceiptStatusRow, 0, 64)
	for rows.Next() {
		var row purchaseReceiptStatusRow
		if err := rows.Scan(
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

func (r *Reader) deliveryNoteStatusRows(ctx context.Context) ([]deliveryNoteStatusRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			dn.docstatus,
			COALESCE(dn.accord_flow_state, 0),
			COALESCE(dn.accord_customer_state, 0)
		FROM `+"`tabDelivery Note`"+` dn
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]deliveryNoteStatusRow, 0, 64)
	for rows.Next() {
		var row deliveryNoteStatusRow
		if err := rows.Scan(
			&row.DocStatus,
			&row.AccordFlowState,
			&row.AccordCustomerState,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Reader) WerkaHome(ctx context.Context, pendingLimit int) (core.WerkaHomeData, error) {
	receipts, err := r.telegramReceiptRows(ctx, "")
	if err != nil {
		return core.WerkaHomeData{}, err
	}
	deliveryNotes, err := r.deliveryNoteRows(ctx, "")
	if err != nil {
		return core.WerkaHomeData{}, err
	}

	data := core.WerkaHomeData{
		Summary:      core.WerkaHomeSummary{},
		PendingItems: make([]core.DispatchRecord, 0, max(pendingLimit, 0)),
	}

	for _, row := range receipts {
		status, include := classifyWerkaReceipt(row)
		if !include {
			continue
		}
		switch status {
		case "pending", "draft":
			data.Summary.PendingCount++
			if pendingLimit <= 0 || len(data.PendingItems) < pendingLimit {
				data.PendingItems = append(data.PendingItems, purchaseReceiptRowToDispatchRecord(row))
			}
		case "accepted":
			data.Summary.ConfirmedCount++
		case "partial", "rejected", "cancelled":
			data.Summary.ReturnedCount++
		}
	}

	for _, row := range deliveryNotes {
		if !deliveryVisible(row) {
			continue
		}
		status := deliveryStatus(row)
		switch status {
		case "pending":
			data.Summary.PendingCount++
			if pendingLimit <= 0 || len(data.PendingItems) < pendingLimit {
				data.PendingItems = append(data.PendingItems, deliveryNoteRowToDispatchRecord(row))
			}
		case "accepted":
			data.Summary.ConfirmedCount++
		case "partial", "rejected", "cancelled":
			data.Summary.ReturnedCount++
		}
	}

	sort.Slice(data.PendingItems, func(i, j int) bool {
		return data.PendingItems[i].CreatedLabel > data.PendingItems[j].CreatedLabel
	})
	if pendingLimit > 0 && len(data.PendingItems) > pendingLimit {
		data.PendingItems = data.PendingItems[:pendingLimit]
	}
	return data, nil
}

func (r *Reader) WerkaStatusBreakdown(ctx context.Context, kind string) ([]core.WerkaStatusBreakdownEntry, error) {
	receipts, err := r.telegramReceiptRows(ctx, "")
	if err != nil {
		return nil, err
	}
	deliveryNotes, err := r.deliveryNoteRows(ctx, "")
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]*core.WerkaStatusBreakdownEntry)
	for _, row := range receipts {
		record := purchaseReceiptRowToDispatchRecord(row)
		if !matchesWerkaBreakdown(record, kind) {
			continue
		}
		key := strings.TrimSpace(record.SupplierRef)
		if key == "" {
			key = strings.TrimSpace(record.SupplierName)
		}
		entry := grouped[key]
		if entry == nil {
			entry = &core.WerkaStatusBreakdownEntry{
				SupplierRef:  record.SupplierRef,
				SupplierName: record.SupplierName,
				UOM:          record.UOM,
			}
			grouped[key] = entry
		}
		entry.ReceiptCount++
		entry.TotalSentQty += record.SentQty
		entry.TotalAcceptedQty += record.AcceptedQty
		entry.TotalReturnedQty += floatMax(record.SentQty-record.AcceptedQty, 0)
		if entry.UOM == "" {
			entry.UOM = record.UOM
		}
	}

	for _, row := range deliveryNotes {
		record := deliveryNoteRowToDispatchRecord(row)
		if !matchesWerkaBreakdown(record, kind) {
			continue
		}
		key := strings.TrimSpace(record.SupplierRef)
		if key == "" {
			key = strings.TrimSpace(record.SupplierName)
		}
		entry := grouped[key]
		if entry == nil {
			entry = &core.WerkaStatusBreakdownEntry{
				SupplierRef:  record.SupplierRef,
				SupplierName: record.SupplierName,
				UOM:          record.UOM,
			}
			grouped[key] = entry
		}
		entry.ReceiptCount++
		entry.TotalSentQty += record.SentQty
		entry.TotalAcceptedQty += record.AcceptedQty
		entry.TotalReturnedQty += floatMax(record.SentQty-record.AcceptedQty, 0)
		if entry.UOM == "" {
			entry.UOM = record.UOM
		}
	}

	result := make([]core.WerkaStatusBreakdownEntry, 0, len(grouped))
	for _, entry := range grouped {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ReceiptCount != result[j].ReceiptCount {
			return result[i].ReceiptCount > result[j].ReceiptCount
		}
		return strings.ToLower(result[i].SupplierName) < strings.ToLower(result[j].SupplierName)
	})
	return result, nil
}

func (r *Reader) WerkaStatusDetails(ctx context.Context, kind, supplierRef string) ([]core.DispatchRecord, error) {
	receipts, err := r.telegramReceiptRows(ctx, "")
	if err != nil {
		return nil, err
	}
	deliveryNotes, err := r.deliveryNoteRows(ctx, "")
	if err != nil {
		return nil, err
	}

	needle := strings.TrimSpace(supplierRef)
	result := make([]core.DispatchRecord, 0, 64)
	for _, row := range receipts {
		record := purchaseReceiptRowToDispatchRecord(row)
		if needle != "" && !strings.EqualFold(strings.TrimSpace(record.SupplierRef), needle) {
			continue
		}
		if !matchesWerkaBreakdown(record, kind) {
			continue
		}
		result = append(result, record)
	}
	for _, row := range deliveryNotes {
		record := deliveryNoteRowToDispatchRecord(row)
		if needle != "" && !strings.EqualFold(strings.TrimSpace(record.SupplierRef), needle) {
			continue
		}
		if !matchesWerkaBreakdown(record, kind) {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	return result, nil
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

func (r *Reader) WerkaHistory(ctx context.Context) ([]core.DispatchRecord, error) {
	const recentLimit = 120
	receipts, err := r.telegramReceiptRowsLimited(ctx, "", recentLimit)
	if err != nil {
		return nil, err
	}
	result := make([]core.DispatchRecord, 0, len(receipts))
	for _, row := range receipts {
		record := purchaseReceiptRowToDispatchRecord(row)
		if record.EventType == "werka_unannounced_pending" {
			continue
		}
		result = append(result, record)
	}

	acks, err := r.supplierAckEventsLimited(ctx, recentLimit)
	if err != nil {
		return nil, err
	}
	result = append(result, acks...)

	customerEvents, err := r.customerResultEventsLimited(ctx, recentLimit)
	if err != nil {
		return nil, err
	}
	result = append(result, customerEvents...)

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedLabel > result[j].CreatedLabel
	})
	if len(result) > recentLimit {
		result = result[:recentLimit]
	}
	return result, nil
}

func (r *Reader) telegramReceiptRows(ctx context.Context, supplierRef string) ([]purchaseReceiptSummaryRow, error) {
	return r.telegramReceiptRowsLimited(ctx, supplierRef, 0)
}

func (r *Reader) telegramReceiptRowsLimited(ctx context.Context, supplierRef string, limit int) ([]purchaseReceiptSummaryRow, error) {
	limit = clampLimit(limit, 0, 1000)
	query := `
		SELECT
			pr.name,
			pr.supplier,
			COALESCE(pr.supplier_name, ''),
			pr.docstatus,
			COALESCE(pr.status, ''),
			COALESCE(pr.total_qty, 0),
			COALESCE(CAST(pr.posting_date AS CHAR), ''),
			COALESCE(pr.supplier_delivery_note, ''),
			COALESCE(pr.remarks, ''),
			COALESCE(pr.currency, ''),
			COALESCE(pri.item_code, ''),
			COALESCE(pri.item_name, ''),
			COALESCE(pri.uom, ''),
			COALESCE(pri.amount, 0)
		FROM ` + "`tabPurchase Receipt`" + ` pr
		LEFT JOIN ` + "`tabPurchase Receipt Item`" + ` pri ON pri.parent = pr.name AND pri.idx = 1
		WHERE pr.supplier_delivery_note LIKE 'TG:%'
		  AND (? = '' OR pr.supplier = ?)
		ORDER BY pr.name DESC`
	if limit > 0 {
		query += "\n\t\tLIMIT ?"
	}
	args := []interface{}{strings.TrimSpace(supplierRef), strings.TrimSpace(supplierRef)}
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]purchaseReceiptSummaryRow, 0, 64)
	for rows.Next() {
		var row purchaseReceiptSummaryRow
		if err := rows.Scan(
			&row.Name,
			&row.Supplier,
			&row.SupplierName,
			&row.DocStatus,
			&row.Status,
			&row.TotalQty,
			&row.PostingDate,
			&row.SupplierDeliveryNote,
			&row.Remarks,
			&row.Currency,
			&row.ItemCode,
			&row.ItemName,
			&row.UOM,
			&row.Amount,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Reader) deliveryNoteRows(ctx context.Context, customerRef string) ([]deliveryNoteSummaryRow, error) {
	return r.deliveryNoteRowsLimited(ctx, customerRef, 0)
}

func (r *Reader) deliveryNoteRowsLimited(ctx context.Context, customerRef string, limit int) ([]deliveryNoteSummaryRow, error) {
	limit = clampLimit(limit, 0, 1000)
	query := `
		SELECT
			dn.name,
			dn.customer,
			COALESCE(dn.customer_name, ''),
			dn.docstatus,
			COALESCE(CAST(dn.modified AS CHAR), ''),
			COALESCE(dn.total_qty, 0),
			COALESCE(dni.returned_qty, 0),
			COALESCE(dn.accord_customer_reason, ''),
			COALESCE(dni.item_code, ''),
			COALESCE(dni.item_name, ''),
			COALESCE(dni.uom, ''),
			COALESCE(dn.accord_flow_state, 0),
			COALESCE(dn.accord_customer_state, 0)
		FROM ` + "`tabDelivery Note`" + ` dn
		LEFT JOIN ` + "`tabDelivery Note Item`" + ` dni ON dni.parent = dn.name AND dni.idx = 1
		WHERE (? = '' OR dn.customer = ?)
		ORDER BY dn.name DESC`
	if limit > 0 {
		query += "\n\t\tLIMIT ?"
	}
	args := []interface{}{strings.TrimSpace(customerRef), strings.TrimSpace(customerRef)}
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]deliveryNoteSummaryRow, 0, 64)
	for rows.Next() {
		var row deliveryNoteSummaryRow
		if err := rows.Scan(
			&row.Name,
			&row.Customer,
			&row.CustomerName,
			&row.DocStatus,
			&row.Modified,
			&row.Qty,
			&row.ReturnedQty,
			&row.CustomerReason,
			&row.ItemCode,
			&row.ItemName,
			&row.UOM,
			&row.AccordFlowState,
			&row.AccordCustomerState,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Reader) supplierAckEvents(ctx context.Context) ([]core.DispatchRecord, error) {
	return r.supplierAckEventsLimited(ctx, 0)
}

func (r *Reader) supplierAckEventsLimited(ctx context.Context, limit int) ([]core.DispatchRecord, error) {
	limit = clampLimit(limit, 0, 1000)
	query := `
		SELECT
			c.name,
			COALESCE(CAST(c.creation AS CHAR), ''),
			pr.supplier,
			COALESCE(pr.supplier_name, ''),
			COALESCE(pr.total_qty, 0),
			COALESCE(pri.item_code, ''),
			COALESCE(pri.item_name, ''),
			COALESCE(pri.uom, '')
		FROM ` + "`tabComment`" + ` c
		INNER JOIN ` + "`tabPurchase Receipt`" + ` pr ON pr.name = c.reference_name
		LEFT JOIN ` + "`tabPurchase Receipt Item`" + ` pri ON pri.parent = pr.name AND pri.idx = 1
		WHERE c.reference_doctype = 'Purchase Receipt'
		  AND c.content LIKE 'Supplier%'
		  AND c.content LIKE '%Tasdiqlayman%'
		ORDER BY c.name DESC`
	if limit > 0 {
		query += "\n\t\tLIMIT ?"
	}
	args := []interface{}{}
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.DispatchRecord, 0, 32)
	for rows.Next() {
		var (
			commentID    string
			createdLabel string
			supplierRef  string
			supplierName string
			sentQty      float64
			itemCode     string
			itemName     string
			uom          string
		)
		if err := rows.Scan(
			&commentID,
			&createdLabel,
			&supplierRef,
			&supplierName,
			&sentQty,
			&itemCode,
			&itemName,
			&uom,
		); err != nil {
			return nil, err
		}
		result = append(result, core.DispatchRecord{
			ID:           supplierAckEventPrefixDB + commentID,
			SupplierRef:  strings.TrimSpace(supplierRef),
			SupplierName: strings.TrimSpace(supplierName),
			ItemCode:     strings.TrimSpace(itemCode),
			ItemName:     strings.TrimSpace(itemName),
			UOM:          strings.TrimSpace(uom),
			SentQty:      sentQty,
			AcceptedQty:  sentQty,
			EventType:    "supplier_ack",
			Highlight:    "Supplier mahsulotni qaytarganingizni tasdiqladi",
			Status:       "accepted",
			CreatedLabel: strings.TrimSpace(createdLabel),
		})
	}
	return result, rows.Err()
}

func (r *Reader) customerResultEvents(ctx context.Context) ([]core.DispatchRecord, error) {
	return r.customerResultEventsLimited(ctx, 0)
}

func (r *Reader) customerResultEventsLimited(ctx context.Context, limit int) ([]core.DispatchRecord, error) {
	rows, err := r.deliveryNoteRowsLimited(ctx, "", limit)
	if err != nil {
		return nil, err
	}
	result := make([]core.DispatchRecord, 0, len(rows))
	for _, row := range rows {
		record, ok := buildCustomerResultDispatch(row)
		if ok {
			result = append(result, record)
		}
	}
	return result, nil
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

func classifyWerkaReceiptStatusRow(row purchaseReceiptStatusRow) (string, bool) {
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

func deliveryStatusVisible(row deliveryNoteStatusRow) bool {
	return row.DocStatus == 1 && row.AccordFlowState == deliveryFlowStateSubmittedDB
}

func deliveryStatus(row deliveryNoteSummaryRow) string {
	return deliveryStatusFromState(row.DocStatus, row.AccordFlowState, row.AccordCustomerState)
}

func deliveryStatusFromState(docStatus, flowState, customerState int) string {
	if docStatus != 1 {
		return "draft"
	}
	if flowState != deliveryFlowStateSubmittedDB {
		return "pending"
	}
	switch customerState {
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

func buildCustomerResultDispatch(row deliveryNoteSummaryRow) (core.DispatchRecord, bool) {
	status := deliveryStatus(row)
	if status != "accepted" && status != "partial" && status != "rejected" {
		return core.DispatchRecord{}, false
	}
	record := deliveryNoteRowToDispatchRecord(row)
	record.ID = customerDeliveryResultEventPrefix(record.ID)
	switch status {
	case "accepted":
		record.EventType = "customer_delivery_confirmed"
		record.Highlight = "Customer mahsulotni qabul qildi"
	case "partial":
		record.EventType = "customer_delivery_partial"
		record.Highlight = "Customer mahsulotning bir qismini qaytardi"
	case "rejected":
		record.EventType = "customer_delivery_rejected"
		record.Highlight = "Customer mahsulotni rad etdi"
	}
	return record, true
}

func customerDeliveryResultEventPrefix(id string) string {
	return "customer_delivery_result:" + strings.TrimSpace(id)
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

func matchesWerkaBreakdown(record core.DispatchRecord, kind string) bool {
	switch strings.TrimSpace(kind) {
	case "pending":
		return record.Status == "pending" || record.Status == "draft"
	case "confirmed":
		return record.Status == "accepted"
	case "returned":
		return record.Status == "partial" || record.Status == "rejected" || record.Status == "cancelled"
	default:
		return false
	}
}

func floatMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func purchaseReceiptRowToDispatchRecord(row purchaseReceiptSummaryRow) core.DispatchRecord {
	sentQty := row.TotalQty
	if markerQty, ok := erpnext.ParseTelegramReceiptMarkerQty(row.SupplierDeliveryNote); ok && markerQty > sentQty {
		sentQty = markerQty
	}
	status, _ := classifyWerkaReceipt(row)
	acceptedQty := 0.0
	switch status {
	case "accepted":
		acceptedQty = row.TotalQty
	case "partial":
		acceptedQty = row.TotalQty
	}
	return core.DispatchRecord{
		ID:           strings.TrimSpace(row.Name),
		RecordType:   "purchase_receipt",
		SupplierRef:  strings.TrimSpace(row.Supplier),
		SupplierName: strings.TrimSpace(row.SupplierName),
		ItemCode:     strings.TrimSpace(row.ItemCode),
		ItemName:     strings.TrimSpace(row.ItemName),
		UOM:          strings.TrimSpace(row.UOM),
		SentQty:      sentQty,
		AcceptedQty:  acceptedQty,
		Amount:       row.Amount,
		Currency:     strings.TrimSpace(row.Currency),
		Status:       status,
		CreatedLabel: strings.TrimSpace(row.PostingDate),
	}
}

func deliveryNoteRowToDispatchRecord(row deliveryNoteSummaryRow) core.DispatchRecord {
	status := deliveryStatus(row)
	acceptedQty, returnedQty := deliveryNoteDecisionQuantities(row, status)
	if status == "accepted" && acceptedQty <= 0 {
		acceptedQty = row.Qty
	}
	if status == "partial" && acceptedQty <= 0 && returnedQty > 0 {
		acceptedQty = floatMax(row.Qty-returnedQty, 0)
	}
	return core.DispatchRecord{
		ID:           strings.TrimSpace(row.Name),
		RecordType:   "delivery_note",
		SupplierRef:  strings.TrimSpace(row.Customer),
		SupplierName: strings.TrimSpace(row.CustomerName),
		ItemCode:     strings.TrimSpace(row.ItemCode),
		ItemName:     strings.TrimSpace(row.ItemName),
		UOM:          strings.TrimSpace(row.UOM),
		SentQty:      row.Qty,
		AcceptedQty:  acceptedQty,
		Status:       status,
		CreatedLabel: strings.TrimSpace(row.Modified),
	}
}

func deliveryNoteDecisionQuantities(row deliveryNoteSummaryRow, status string) (acceptedQty, returnedQty float64) {
	switch status {
	case "accepted":
		return row.Qty, 0
	case "partial":
		returnedQty = row.ReturnedQty
		if returnedQty <= 0 {
			returnedQty = floatMax(row.Qty, 0)
		}
		acceptedQty = floatMax(row.Qty-returnedQty, 0)
		return acceptedQty, returnedQty
	case "rejected", "cancelled":
		return 0, row.Qty
	default:
		return 0, 0
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

func (r *Reader) loadAllWerkaCustomerItems(ctx context.Context, customerRef string) ([]core.SupplierItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
		FROM `+"`tabItem Customer Detail`"+` icd
		INNER JOIN tabItem i ON i.name = icd.parent
		WHERE icd.customer_name = ?
		  AND i.disabled = 0
		ORDER BY i.item_name ASC`,
		strings.TrimSpace(customerRef),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.SupplierItem, 0, 64)
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

func (r *Reader) loadAllWerkaCustomerItemSearchEntries(ctx context.Context, customerRef string) ([]supplierItemSearchEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom,
			COALESCE(GROUP_CONCAT(DISTINCT icd_all.customer_name ORDER BY icd_all.customer_name SEPARATOR '\n'), '') AS customer_refs,
			COALESCE(GROUP_CONCAT(DISTINCT COALESCE(NULLIF(c.customer_name, ''), c.name) ORDER BY COALESCE(NULLIF(c.customer_name, ''), c.name) SEPARATOR '\n'), '') AS customer_names
		FROM `+"`tabItem Customer Detail`"+` icd_selected
		INNER JOIN tabItem i ON i.name = icd_selected.parent
		LEFT JOIN `+"`tabItem Customer Detail`"+` icd_all ON icd_all.parent = i.name
		LEFT JOIN tabCustomer c ON c.name = icd_all.customer_name
		WHERE icd_selected.customer_name = ?
		  AND i.disabled = 0
		GROUP BY i.item_code, item_name, stock_uom
		ORDER BY item_name ASC`,
		strings.TrimSpace(customerRef),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]supplierItemSearchEntry, 0, 64)
	for rows.Next() {
		var (
			item          core.SupplierItem
			customerRefs  string
			customerNames string
		)
		if err := rows.Scan(&item.Code, &item.Name, &item.UOM, &customerRefs, &customerNames); err != nil {
			return nil, err
		}
		item.Warehouse = r.defaultWarehouse
		searchTerms := []string{item.Code, item.Name}
		searchTerms = appendSearchTerms(searchTerms, customerRefs)
		searchTerms = appendSearchTerms(searchTerms, customerNames)
		result = append(result, supplierItemSearchEntry{
			item:        item,
			searchTerms: searchTerms,
		})
	}
	return result, rows.Err()
}

func (r *Reader) loadAllWerkaSupplierItems(ctx context.Context, supplierRef string) ([]core.SupplierItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT
			i.item_code,
			COALESCE(NULLIF(i.item_name, ''), i.item_code) AS item_name,
			COALESCE(NULLIF(i.stock_uom, ''), 'Nos') AS stock_uom
		FROM `+"`tabItem Supplier`"+` isup
		INNER JOIN tabItem i ON i.name = isup.parent
		WHERE isup.supplier = ?
		  AND i.disabled = 0
		ORDER BY i.item_name ASC`,
		strings.TrimSpace(supplierRef),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.SupplierItem, 0, 64)
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

func (r *Reader) loadAllWerkaCustomerItemOptions(ctx context.Context) ([]core.CustomerItemOption, error) {
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
		ORDER BY i.item_name ASC, c.customer_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.CustomerItemOption, 0, 128)
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

func rankSupplierItemsByQuery(items []core.SupplierItem, query string) []core.SupplierItem {
	if strings.TrimSpace(query) == "" {
		return items
	}

	type scoredItem struct {
		item  core.SupplierItem
		score int
	}

	scored := make([]scoredItem, 0, len(items))
	for _, item := range items {
		score := erpnext.SearchQueryScore(query, item.Code, item.Name)
		if score == 0 {
			continue
		}
		scored = append(scored, scoredItem{item: item, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		leftName := strings.ToLower(strings.TrimSpace(scored[i].item.Name))
		rightName := strings.ToLower(strings.TrimSpace(scored[j].item.Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToLower(strings.TrimSpace(scored[i].item.Code)) < strings.ToLower(strings.TrimSpace(scored[j].item.Code))
	})

	result := make([]core.SupplierItem, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.item)
	}
	return result
}

func rankSupplierItemSearchEntriesByQuery(items []supplierItemSearchEntry, query string) []core.SupplierItem {
	if strings.TrimSpace(query) == "" {
		result := make([]core.SupplierItem, 0, len(items))
		for _, item := range items {
			result = append(result, item.item)
		}
		return result
	}

	type scoredItem struct {
		item  core.SupplierItem
		score int
	}

	scored := make([]scoredItem, 0, len(items))
	for _, item := range items {
		score := erpnext.SearchQueryScore(query, item.searchTerms...)
		if score == 0 {
			continue
		}
		scored = append(scored, scoredItem{item: item.item, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		leftName := strings.ToLower(strings.TrimSpace(scored[i].item.Name))
		rightName := strings.ToLower(strings.TrimSpace(scored[j].item.Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToLower(strings.TrimSpace(scored[i].item.Code)) < strings.ToLower(strings.TrimSpace(scored[j].item.Code))
	})

	result := make([]core.SupplierItem, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.item)
	}
	return result
}

func appendSearchTerms(terms []string, joined string) []string {
	for _, part := range strings.Split(joined, "\n") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		terms = append(terms, trimmed)
	}
	return terms
}

func rankCustomerItemOptionsByQuery(items []core.CustomerItemOption, query string) []core.CustomerItemOption {
	if strings.TrimSpace(query) == "" {
		return items
	}

	type scoredOption struct {
		item          core.CustomerItemOption
		itemScore     int
		customerScore int
	}

	scored := make([]scoredOption, 0, len(items))
	for _, item := range items {
		itemScore := erpnext.SearchQueryScore(query, item.ItemCode, item.ItemName)
		customerScore := erpnext.SearchQueryScore(query, item.CustomerName, item.CustomerRef, item.CustomerPhone)
		if itemScore == 0 && customerScore == 0 {
			continue
		}
		scored = append(scored, scoredOption{
			item:          item,
			itemScore:     itemScore,
			customerScore: customerScore,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].itemScore != scored[j].itemScore {
			return scored[i].itemScore > scored[j].itemScore
		}
		if scored[i].customerScore != scored[j].customerScore {
			return scored[i].customerScore > scored[j].customerScore
		}
		leftName := strings.ToLower(strings.TrimSpace(scored[i].item.ItemName))
		rightName := strings.ToLower(strings.TrimSpace(scored[j].item.ItemName))
		if leftName != rightName {
			return leftName < rightName
		}
		leftCustomer := strings.ToLower(strings.TrimSpace(scored[i].item.CustomerName))
		rightCustomer := strings.ToLower(strings.TrimSpace(scored[j].item.CustomerName))
		if leftCustomer != rightCustomer {
			return leftCustomer < rightCustomer
		}
		return strings.ToLower(strings.TrimSpace(scored[i].item.ItemCode)) < strings.ToLower(strings.TrimSpace(scored[j].item.ItemCode))
	})

	result := make([]core.CustomerItemOption, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.item)
	}
	return result
}

func slicePage[T any](items []T, offset, limit int) []T {
	start := max(offset, 0)
	if start >= len(items) {
		return []T{}
	}

	end := len(items)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	result := make([]T, end-start)
	copy(result, items[start:end])
	return result
}

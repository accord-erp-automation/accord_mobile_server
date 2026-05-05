package erpdb

import (
	"context"
	"fmt"
	"strings"

	"mobile_server/internal/core"
)

type stockEntryBarcodeRow struct {
	stockEntryName  string
	stockEntryType  string
	docStatus       int
	status          string
	company         string
	postingDate     string
	postingTime     string
	creation        string
	modified        string
	remarks         string
	lineIndex       int
	itemCode        string
	itemName        string
	qty             float64
	uom             string
	stockUOM        string
	barcode         string
	sourceWarehouse string
	targetWarehouse string
}

func (r *Reader) StockEntriesByBarcode(ctx context.Context, barcode string, limit int) ([]core.StockEntryBarcodeEntry, error) {
	limit = clampLimit(limit, 20, 50)
	normalized := strings.ToUpper(strings.TrimSpace(barcode))
	if normalized == "" {
		return nil, fmt.Errorf("barcode is required")
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			se.name,
			COALESCE(se.stock_entry_type, ''),
			COALESCE(se.docstatus, 0),
			CASE COALESCE(se.docstatus, 0)
				WHEN 0 THEN 'Draft'
				WHEN 1 THEN 'Submitted'
				WHEN 2 THEN 'Cancelled'
				ELSE ''
			END,
			COALESCE(se.company, ''),
			COALESCE(CAST(se.posting_date AS CHAR), ''),
			COALESCE(CAST(se.posting_time AS CHAR), ''),
			COALESCE(CAST(se.creation AS CHAR), ''),
			COALESCE(CAST(se.modified AS CHAR), ''),
			COALESCE(se.remarks, ''),
			COALESCE(sed.idx, 0),
			COALESCE(sed.item_code, ''),
			COALESCE(NULLIF(i.item_name, ''), sed.item_code, ''),
			COALESCE(sed.qty, 0),
			COALESCE(NULLIF(sed.uom, ''), NULLIF(sed.stock_uom, ''), ''),
			COALESCE(NULLIF(sed.stock_uom, ''), NULLIF(sed.uom, ''), ''),
			COALESCE(sed.barcode, ''),
			COALESCE(NULLIF(sed.s_warehouse, ''), NULLIF(se.from_warehouse, ''), ''),
			COALESCE(NULLIF(sed.t_warehouse, ''), NULLIF(se.to_warehouse, ''), '')
		FROM `+"`tabStock Entry Detail`"+` sed
		INNER JOIN `+"`tabStock Entry`"+` se ON se.name = sed.parent
		LEFT JOIN tabItem i ON i.name = sed.item_code
		WHERE COALESCE(sed.barcode, '') = ?
		ORDER BY se.modified DESC, se.creation DESC, se.name DESC, sed.idx ASC
		LIMIT ?`,
		normalized,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]core.StockEntryBarcodeEntry, 0, limit)
	for rows.Next() {
		var row stockEntryBarcodeRow
		if err := rows.Scan(
			&row.stockEntryName,
			&row.stockEntryType,
			&row.docStatus,
			&row.status,
			&row.company,
			&row.postingDate,
			&row.postingTime,
			&row.creation,
			&row.modified,
			&row.remarks,
			&row.lineIndex,
			&row.itemCode,
			&row.itemName,
			&row.qty,
			&row.uom,
			&row.stockUOM,
			&row.barcode,
			&row.sourceWarehouse,
			&row.targetWarehouse,
		); err != nil {
			return nil, err
		}
		result = append(result, core.StockEntryBarcodeEntry{
			StockEntryName:  strings.TrimSpace(row.stockEntryName),
			StockEntryType:  strings.TrimSpace(row.stockEntryType),
			DocStatus:       row.docStatus,
			Status:          strings.TrimSpace(row.status),
			Company:         strings.TrimSpace(row.company),
			PostingDate:     strings.TrimSpace(row.postingDate),
			PostingTime:     strings.TrimSpace(row.postingTime),
			Creation:        strings.TrimSpace(row.creation),
			Modified:        strings.TrimSpace(row.modified),
			Remarks:         strings.TrimSpace(row.remarks),
			LineIndex:       row.lineIndex,
			ItemCode:        strings.TrimSpace(row.itemCode),
			ItemName:        strings.TrimSpace(row.itemName),
			Qty:             row.qty,
			UOM:             strings.TrimSpace(row.uom),
			StockUOM:        strings.TrimSpace(row.stockUOM),
			Barcode:         strings.TrimSpace(row.barcode),
			SourceWarehouse: strings.TrimSpace(row.sourceWarehouse),
			TargetWarehouse: strings.TrimSpace(row.targetWarehouse),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, core.ErrStockEntryNotFound
	}
	return result, nil
}

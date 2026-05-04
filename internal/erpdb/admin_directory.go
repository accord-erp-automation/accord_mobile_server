package erpdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mobile_server/internal/core"
)

func (r *Reader) AdminSupplierDirectoryPage(ctx context.Context, query string, limit, offset int) ([]core.SupplierDirectoryEntry, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			s.name,
			COALESCE(NULLIF(s.supplier_name, ''), s.name) AS supplier_name,
			COALESCE(s.mobile_no, '')
		FROM tabSupplier s
		WHERE s.disabled = 0
		  AND (? = '' OR s.name LIKE ? ESCAPE '\\' OR s.supplier_name LIKE ? ESCAPE '\\' OR COALESCE(s.mobile_no, '') LIKE ? ESCAPE '\\')
		ORDER BY s.modified DESC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query),
		like,
		like,
		like,
		limit,
		max(offset, 0),
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

func (r *Reader) AdminSupplierByRef(ctx context.Context, ref string) (core.SupplierDirectoryEntry, error) {
	var item core.SupplierDirectoryEntry
	err := r.db.QueryRowContext(ctx, `
		SELECT
			s.name,
			COALESCE(NULLIF(s.supplier_name, ''), s.name) AS supplier_name,
			COALESCE(s.mobile_no, '')
		FROM tabSupplier s
		WHERE s.disabled = 0
		  AND s.name = ?
		LIMIT 1`,
		strings.TrimSpace(ref),
	).Scan(&item.Ref, &item.Name, &item.Phone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.SupplierDirectoryEntry{}, fmt.Errorf("supplier not found")
		}
		return core.SupplierDirectoryEntry{}, err
	}
	return item, nil
}

func (r *Reader) AdminCustomerDirectoryPage(ctx context.Context, query string, limit, offset int) ([]core.CustomerDirectoryEntry, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.name,
			COALESCE(NULLIF(c.customer_name, ''), c.name) AS customer_name,
			COALESCE(c.mobile_no, '')
		FROM tabCustomer c
		WHERE c.disabled = 0
		  AND (? = '' OR c.name LIKE ? ESCAPE '\\' OR c.customer_name LIKE ? ESCAPE '\\' OR COALESCE(c.mobile_no, '') LIKE ? ESCAPE '\\')
		ORDER BY c.modified DESC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query),
		like,
		like,
		like,
		limit,
		max(offset, 0),
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

func (r *Reader) AdminCustomerByRef(ctx context.Context, ref string) (core.CustomerDirectoryEntry, error) {
	var item core.CustomerDirectoryEntry
	err := r.db.QueryRowContext(ctx, `
		SELECT
			c.name,
			COALESCE(NULLIF(c.customer_name, ''), c.name) AS customer_name,
			COALESCE(c.mobile_no, '')
		FROM tabCustomer c
		WHERE c.disabled = 0
		  AND c.name = ?
		LIMIT 1`,
		strings.TrimSpace(ref),
	).Scan(&item.Ref, &item.Name, &item.Phone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.CustomerDirectoryEntry{}, fmt.Errorf("customer not found")
		}
		return core.CustomerDirectoryEntry{}, err
	}
	return item, nil
}

func (r *Reader) AdminItemGroupsPage(ctx context.Context, query string, limit, offset int) ([]string, error) {
	limit = clampLimit(limit, 50, 500)
	like := likePattern(query)
	rows, err := r.db.QueryContext(ctx, `
		SELECT name
		FROM `+"`tabItem Group`"+`
		WHERE disabled = 0
		  AND (? = '' OR name LIKE ? ESCAPE '\\' OR item_group_name LIKE ? ESCAPE '\\')
		ORDER BY name ASC
		LIMIT ? OFFSET ?`,
		strings.TrimSpace(query),
		like,
		like,
		limit,
		max(offset, 0),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0, limit)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, rows.Err()
}

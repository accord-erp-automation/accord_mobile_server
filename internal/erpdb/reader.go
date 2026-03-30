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

func (r *Reader) SearchWerkaCustomers(ctx context.Context, query string, limit int) ([]core.CustomerDirectoryEntry, error) {
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
		LIMIT ?`,
		strings.TrimSpace(query), like, like, like, limit,
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

func (r *Reader) SearchWerkaCustomerItems(ctx context.Context, customerRef, query string, limit int) ([]core.SupplierItem, error) {
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
		LIMIT ?`,
		strings.TrimSpace(customerRef),
		strings.TrimSpace(query), like, like, limit,
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

func (r *Reader) SearchWerkaCustomerItemOptions(ctx context.Context, query string, limit int) ([]core.CustomerItemOption, error) {
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
		LIMIT ?`,
		strings.TrimSpace(query), like, like, like, like, like, limit,
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

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if max > 0 && value > max {
		value = max
	}
	return value
}

func likePattern(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "%"
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(trimmed) + "%"
}

package core

import (
	"context"
	"strings"

	"mobile_server/internal/erpnext"
)

type adminDirectoryReader interface {
	AdminSupplierDirectoryPage(ctx context.Context, query string, limit, offset int) ([]SupplierDirectoryEntry, error)
	AdminSupplierByRef(ctx context.Context, ref string) (SupplierDirectoryEntry, error)
	AdminCustomerDirectoryPage(ctx context.Context, query string, limit, offset int) ([]CustomerDirectoryEntry, error)
	AdminCustomerByRef(ctx context.Context, ref string) (CustomerDirectoryEntry, error)
	NotificationDetailByReceiptID(ctx context.Context, receiptID string) (NotificationDetail, error)
}

func (a *ERPAuthenticator) adminDirectoryReader() (adminDirectoryReader, bool) {
	reader, ok := a.reader.(adminDirectoryReader)
	return reader, ok
}

func (a *ERPAuthenticator) adminSupplierDirectoryEntries(ctx context.Context, limit int) ([]SupplierDirectoryEntry, error) {
	if reader, ok := a.adminDirectoryReader(); ok {
		const pageSize = 200
		result := make([]SupplierDirectoryEntry, 0, pageSize)
		for offset := 0; ; offset += pageSize {
			pageLimit := pageSize
			if limit > 0 {
				remaining := limit - len(result)
				if remaining <= 0 {
					break
				}
				if remaining < pageLimit {
					pageLimit = remaining
				}
			}
			page, err := reader.AdminSupplierDirectoryPage(ctx, "", pageLimit, offset)
			if err != nil {
				return nil, err
			}
			result = append(result, page...)
			if len(page) < pageLimit {
				break
			}
			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return result, nil
	}

	searchLimit := limit
	if searchLimit <= 0 {
		searchLimit = 500
	}
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", searchLimit)
	if err != nil {
		return nil, err
	}
	result := make([]SupplierDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) adminCustomerDirectoryEntries(ctx context.Context, limit int) ([]CustomerDirectoryEntry, error) {
	if reader, ok := a.adminDirectoryReader(); ok {
		const pageSize = 200
		result := make([]CustomerDirectoryEntry, 0, pageSize)
		for offset := 0; ; offset += pageSize {
			pageLimit := pageSize
			if limit > 0 {
				remaining := limit - len(result)
				if remaining <= 0 {
					break
				}
				if remaining < pageLimit {
					pageLimit = remaining
				}
			}
			page, err := reader.AdminCustomerDirectoryPage(ctx, "", pageLimit, offset)
			if err != nil {
				return nil, err
			}
			result = append(result, page...)
			if len(page) < pageLimit {
				break
			}
			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return result, nil
	}

	searchLimit := limit
	if searchLimit <= 0 {
		searchLimit = 500
	}
	items, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", searchLimit)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, CustomerDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return result, nil
}

func (a *ERPAuthenticator) adminSupplierDirectoryEntriesPage(ctx context.Context, limit, offset int) ([]SupplierDirectoryEntry, error) {
	if reader, ok := a.adminDirectoryReader(); ok {
		return reader.AdminSupplierDirectoryPage(ctx, "", limit, offset)
	}

	searchLimit := limit + maxInt(offset, 0)
	if searchLimit <= 0 {
		searchLimit = 500
	}
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", searchLimit)
	if err != nil {
		return nil, err
	}
	result := make([]SupplierDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return sliceDirectoryEntries(result, offset, limit), nil
}

func (a *ERPAuthenticator) adminCustomerDirectoryEntriesPage(ctx context.Context, limit, offset int) ([]CustomerDirectoryEntry, error) {
	if reader, ok := a.adminDirectoryReader(); ok {
		return reader.AdminCustomerDirectoryPage(ctx, "", limit, offset)
	}

	searchLimit := limit + maxInt(offset, 0)
	if searchLimit <= 0 {
		searchLimit = 500
	}
	items, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", searchLimit)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerDirectoryEntry, 0, len(items))
	for _, item := range items {
		result = append(result, CustomerDirectoryEntry{
			Ref:   item.ID,
			Name:  item.Name,
			Phone: item.Phone,
		})
	}
	return sliceDirectoryEntries(result, offset, limit), nil
}

func (a *ERPAuthenticator) adminSupplierByRef(ctx context.Context, ref string) (supplier erpnext.Supplier, ok bool, err error) {
	if reader, readerOK := a.adminDirectoryReader(); readerOK {
		entry, readerErr := reader.AdminSupplierByRef(ctx, ref)
		if readerErr == nil {
			return erpnext.Supplier{
				ID:    entry.Ref,
				Name:  entry.Name,
				Phone: entry.Phone,
			}, true, nil
		}
	}

	item, err := a.erp.GetSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, ref)
	if err != nil {
		return erpnext.Supplier{}, false, err
	}
	if strings.TrimSpace(item.ID) == "" {
		return erpnext.Supplier{}, false, ErrAdminSupplierNotFound
	}
	return item, false, nil
}

func (a *ERPAuthenticator) adminCustomerByRef(ctx context.Context, ref string) (customer erpnext.Customer, ok bool, err error) {
	if reader, readerOK := a.adminDirectoryReader(); readerOK {
		entry, readerErr := reader.AdminCustomerByRef(ctx, ref)
		if readerErr == nil {
			return erpnext.Customer{
				ID:    entry.Ref,
				Name:  entry.Name,
				Phone: entry.Phone,
			}, true, nil
		}
	}

	item, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, ref)
	if err != nil {
		return erpnext.Customer{}, false, err
	}
	if strings.TrimSpace(item.ID) == "" {
		return erpnext.Customer{}, false, ErrAdminSupplierNotFound
	}
	return item, false, nil
}

func sliceDirectoryEntries[T any](items []T, offset, limit int) []T {
	start := maxInt(offset, 0)
	if start >= len(items) {
		return []T{}
	}
	end := len(items)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	return append([]T(nil), items[start:end]...)
}

func maxInt(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}

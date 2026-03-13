package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

const supplierCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const (
	codeRegenWindow        = time.Minute
	maxCodeRegensPerWindow = 3
	accordCodeLinePrefix   = "Accord Code:"
	werkaStateRef          = "__werka__"
)

var (
	ErrAdminSupplierNotFound = errors.New("admin supplier not found")
	ErrCodeRegenCooldown     = errors.New("code regenerate cooldown")
)

func (a *ERPAuthenticator) AdminSupplierSummary(ctx context.Context, limit int) (AdminSupplierSummary, error) {
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", limit)
	if err != nil {
		return AdminSupplierSummary{}, err
	}
	states, err := a.adminSupplierStates()
	if err != nil {
		return AdminSupplierSummary{}, err
	}

	summary := AdminSupplierSummary{
		TotalSuppliers: len(items),
	}
	for _, item := range items {
		state := states[strings.TrimSpace(item.ID)]
		if state.Blocked || state.Removed {
			summary.BlockedSuppliers++
			continue
		}
		summary.ActiveSuppliers++
	}
	return summary, nil
}

func (a *ERPAuthenticator) AdminSuppliers(ctx context.Context, limit int) ([]AdminSupplier, error) {
	return a.adminSuppliersWithOptions(ctx, limit, false)
}

func (a *ERPAuthenticator) AdminInactiveSuppliers(ctx context.Context, limit int) ([]AdminSupplier, error) {
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", limit)
	if err != nil {
		return nil, err
	}
	states, err := a.adminSupplierStates()
	if err != nil {
		return nil, err
	}

	result := make([]AdminSupplier, 0, len(items))
	for _, item := range items {
		state := states[strings.TrimSpace(item.ID)]
		if !state.Blocked && !state.Removed {
			continue
		}
		adminItem, err := a.buildAdminSupplier(item, state)
		if err != nil {
			continue
		}
		result = append(result, adminItem)
	}
	return result, nil
}

func (a *ERPAuthenticator) adminSuppliersWithOptions(ctx context.Context, limit int, includeRemoved bool) ([]AdminSupplier, error) {
	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", limit)
	if err != nil {
		return nil, err
	}
	states, err := a.adminSupplierStates()
	if err != nil {
		return nil, err
	}

	result := make([]AdminSupplier, 0, len(items))
	for _, item := range items {
		state := states[strings.TrimSpace(item.ID)]
		if state.Removed && !includeRemoved {
			continue
		}

		adminItem, err := a.buildAdminSupplier(item, state)
		if err != nil {
			continue
		}
		result = append(result, adminItem)
	}
	return result, nil
}

func (a *ERPAuthenticator) AdminSupplierDetail(ctx context.Context, ref string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	mappedAssignedItems, err := a.adminAssignedItems(ctx, item.ID, state, 200)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	code, err := a.supplierAccessCode(item, state)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	return AdminSupplierDetail{
		Ref:               item.ID,
		Name:              item.Name,
		Phone:             item.Phone,
		Code:              code,
		Blocked:           state.Blocked,
		Removed:           state.Removed,
		CodeLocked:        state.isCodeLocked(a.nowUTC()),
		CodeRetryAfterSec: state.retryAfterSeconds(a.nowUTC()),
		AssignedItems:     mappedAssignedItems,
	}, nil
}

func (a *ERPAuthenticator) AdminSearchItems(ctx context.Context, query string, limit int) ([]SupplierItem, error) {
	items, err := a.erp.SearchItems(ctx, a.baseURL, a.apiKey, a.apiSecret, query, limit)
	if err != nil {
		return nil, err
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) AdminAssignedSupplierItems(ctx context.Context, ref string, limit int) ([]SupplierItem, error) {
	item, _, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return nil, err
	}
	state, err := a.adminSupplierState(item.ID)
	if err != nil {
		return nil, err
	}
	return a.adminAssignedItems(ctx, item.ID, state, limit)
}

func (a *ERPAuthenticator) AdminAssignSupplierItem(ctx context.Context, ref, itemCode string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	if err := a.erp.AssignSupplierToItem(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(itemCode), item.ID); err != nil {
		return AdminSupplierDetail{}, err
	}
	state.AssignmentsConfigured = true
	state.AssignedItemCodes = append(normalizeItemCodes(state.AssignedItemCodes), strings.TrimSpace(itemCode))
	state.UpdatedAt = a.nowUTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminUnassignSupplierItem(ctx context.Context, ref, itemCode string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	if err := a.erp.RemoveSupplierFromItem(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(itemCode), item.ID); err != nil {
		return AdminSupplierDetail{}, err
	}
	filtered := make([]string, 0, len(state.AssignedItemCodes))
	for _, code := range state.AssignedItemCodes {
		if strings.EqualFold(strings.TrimSpace(code), strings.TrimSpace(itemCode)) {
			continue
		}
		filtered = append(filtered, code)
	}
	state.AssignmentsConfigured = true
	state.AssignedItemCodes = filtered
	state.UpdatedAt = a.nowUTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminCreateItem(ctx context.Context, code, name, uom string) (SupplierItem, error) {
	item, err := a.erp.CreateItem(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateItemInput{
		Code: strings.TrimSpace(code),
		Name: strings.TrimSpace(name),
		UOM:  strings.TrimSpace(uom),
	})
	if err != nil {
		return SupplierItem{}, err
	}

	items, err := a.mapSupplierItems(ctx, []erpnext.Item{item})
	if err != nil {
		return SupplierItem{}, err
	}
	if len(items) == 0 {
		return SupplierItem{}, fmt.Errorf("item create returned empty result")
	}
	return items[0], nil
}

func (a *ERPAuthenticator) AdminUpdateSupplierItems(ctx context.Context, ref string, itemCodes []string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	normalizedCodes := normalizeItemCodes(itemCodes)
	if len(normalizedCodes) > 0 {
		items, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, normalizedCodes)
		if err != nil {
			return AdminSupplierDetail{}, err
		}
		found := make(map[string]struct{}, len(items))
		for _, item := range items {
			found[strings.TrimSpace(item.Code)] = struct{}{}
		}
		for _, code := range normalizedCodes {
			if _, ok := found[code]; !ok {
				return AdminSupplierDetail{}, fmt.Errorf("item topilmadi: %s", code)
			}
		}
	}

	currentItems, err := a.erp.ListAssignedSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, item.ID, 200)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	currentCodes := make(map[string]struct{}, len(currentItems))
	for _, current := range currentItems {
		currentCodes[strings.TrimSpace(current.Code)] = struct{}{}
	}
	desiredCodes := make(map[string]struct{}, len(normalizedCodes))
	for _, code := range normalizedCodes {
		desiredCodes[code] = struct{}{}
		if _, ok := currentCodes[code]; !ok {
			if err := a.erp.AssignSupplierToItem(ctx, a.baseURL, a.apiKey, a.apiSecret, code, item.ID); err != nil {
				return AdminSupplierDetail{}, err
			}
		}
	}
	for code := range currentCodes {
		if _, ok := desiredCodes[code]; ok {
			continue
		}
		if err := a.erp.RemoveSupplierFromItem(ctx, a.baseURL, a.apiKey, a.apiSecret, code, item.ID); err != nil {
			return AdminSupplierDetail{}, err
		}
	}

	state.AssignmentsConfigured = true
	state.AssignedItemCodes = normalizedCodes
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminSetSupplierBlocked(ctx context.Context, ref string, blocked bool) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	state.Blocked = blocked
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminUpdateSupplierPhone(ctx context.Context, ref, phone string) (AdminSupplierDetail, error) {
	item, _, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	cleanPhone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(phone)
	if !strings.HasPrefix(strings.TrimSpace(cleanPhone), "+") {
		digitsOnly := cleanPhone
		if len(digitsOnly) == 9 {
			cleanPhone = "998" + digitsOnly
		}
	}
	normalizedPhone, err := suplier.NormalizePhone(cleanPhone)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	details := upsertSupplierPhoneInDetails(item.Details, normalizedPhone)
	if err := a.erp.UpdateSupplierContact(ctx, a.baseURL, a.apiKey, a.apiSecret, item.ID, normalizedPhone, details); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminRegenerateSupplierCode(ctx context.Context, ref string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	items, err := a.erp.SearchSuppliers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", 500)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	states, err := a.adminSupplierStates()
	if err != nil {
		return AdminSupplierDetail{}, err
	}

	existingCodes := make(map[string]struct{}, len(items))
	for _, candidate := range items {
		candidateState := states[strings.TrimSpace(candidate.ID)]
		if candidateState.Removed {
			continue
		}
		code, err := a.supplierAccessCode(candidate, candidateState)
		if err != nil {
			continue
		}
		existingCodes[code] = struct{}{}
	}

	state.CustomCode, err = randomSupplierCode(a.supplierPrefix, existingCodes)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	now := a.nowUTC()
	state, err = a.bumpCodeRegenState(state, now)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	state.PendingPersistCode = state.CustomCode
	state.PendingPersistAt = now.Add(codeRegenWindow)
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	a.scheduleSupplierCodePersist(item.ID, state.CustomCode, state.PendingPersistAt)
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminRemoveSupplier(ctx context.Context, ref string) error {
	item, state, err := a.findSupplierForAdmin(ctx, ref)
	if err != nil {
		return err
	}
	state.Removed = true
	state.Blocked = true
	state.UpdatedAt = time.Now().UTC()
	return a.saveAdminSupplierState(item.ID, state)
}

func (a *ERPAuthenticator) AdminRestoreSupplier(ctx context.Context, ref string) (AdminSupplierDetail, error) {
	item, state, err := a.findSupplierForAdminIncludingRemoved(ctx, ref)
	if err != nil {
		return AdminSupplierDetail{}, err
	}
	state.Removed = false
	state.Blocked = false
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminSupplierDetail{}, err
	}
	return a.AdminSupplierDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminCreateSupplier(ctx context.Context, name, phone string) (AdminSupplier, error) {
	item, err := a.erp.EnsureSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateSupplierInput{
		Name:  strings.TrimSpace(name),
		Phone: strings.TrimSpace(phone),
	})
	if err != nil {
		return AdminSupplier{}, err
	}

	state, err := a.adminSupplierState(item.ID)
	if err != nil {
		return AdminSupplier{}, err
	}
	if state.Removed {
		state.Removed = false
		state.Blocked = false
		state.UpdatedAt = time.Now().UTC()
		if err := a.saveAdminSupplierState(item.ID, state); err != nil {
			return AdminSupplier{}, err
		}
	}

	return a.buildAdminSupplier(item, state)
}

func (a *ERPAuthenticator) AdminCreateCustomer(ctx context.Context, name, phone string) (CustomerDirectoryEntry, error) {
	item, err := a.erp.EnsureCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, erpnext.CreateCustomerInput{
		Name:  strings.TrimSpace(name),
		Phone: strings.TrimSpace(phone),
	})
	if err != nil {
		return CustomerDirectoryEntry{}, err
	}
	return CustomerDirectoryEntry{
		Ref:   item.ID,
		Name:  item.Name,
		Phone: item.Phone,
	}, nil
}

func (a *ERPAuthenticator) AdminCustomers(ctx context.Context, limit int) ([]CustomerDirectoryEntry, error) {
	items, err := a.erp.SearchCustomers(ctx, a.baseURL, a.apiKey, a.apiSecret, "", limit)
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

func (a *ERPAuthenticator) AdminCustomerDetail(ctx context.Context, ref string) (AdminCustomerDetail, error) {
	item, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(ref))
	if err != nil {
		return AdminCustomerDetail{}, err
	}
	state, err := a.adminSupplierState(item.ID)
	if err != nil {
		return AdminCustomerDetail{}, err
	}
	code := strings.TrimSpace(state.CustomCode)
	return AdminCustomerDetail{
		Ref:               item.ID,
		Name:              item.Name,
		Phone:             item.Phone,
		Code:              code,
		CodeLocked:        state.isCodeLocked(a.nowUTC()),
		CodeRetryAfterSec: state.retryAfterSeconds(a.nowUTC()),
	}, nil
}

func (a *ERPAuthenticator) AdminUpdateCustomerPhone(ctx context.Context, ref, phone string) (AdminCustomerDetail, error) {
	item, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(ref))
	if err != nil {
		return AdminCustomerDetail{}, err
	}

	cleanPhone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(phone)
	if !strings.HasPrefix(strings.TrimSpace(cleanPhone), "+") {
		digitsOnly := cleanPhone
		if len(digitsOnly) == 9 {
			cleanPhone = "998" + digitsOnly
		}
	}
	normalizedPhone, err := suplier.NormalizePhone(cleanPhone)
	if err != nil {
		return AdminCustomerDetail{}, err
	}

	details := upsertSupplierPhoneInDetails(item.Details, normalizedPhone)
	if err := a.erp.UpdateCustomerContact(ctx, a.baseURL, a.apiKey, a.apiSecret, item.ID, normalizedPhone, details); err != nil {
		return AdminCustomerDetail{}, err
	}
	return a.AdminCustomerDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) AdminRegenerateCustomerCode(ctx context.Context, ref string) (AdminCustomerDetail, error) {
	item, err := a.erp.GetCustomer(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(ref))
	if err != nil {
		return AdminCustomerDetail{}, err
	}
	state, err := a.adminSupplierState(item.ID)
	if err != nil {
		return AdminCustomerDetail{}, err
	}

	existing := map[string]struct{}{}
	if codes, err := a.adminSupplierStates(); err == nil {
		for _, candidate := range codes {
			if trimmed := strings.TrimSpace(candidate.CustomCode); trimmed != "" {
				existing[trimmed] = struct{}{}
			}
		}
	}

	state.CustomCode, err = randomSupplierCode("30", existing)
	if err != nil {
		return AdminCustomerDetail{}, err
	}
	now := a.nowUTC()
	state, err = a.bumpCodeRegenState(state, now)
	if err != nil {
		return AdminCustomerDetail{}, err
	}
	state.UpdatedAt = time.Now().UTC()
	if err := a.saveAdminSupplierState(item.ID, state); err != nil {
		return AdminCustomerDetail{}, err
	}

	details := upsertAccordCodeInDetails(item.Details, state.CustomCode)
	if err := a.erp.UpdateCustomerDetails(ctx, a.baseURL, a.apiKey, a.apiSecret, item.ID, details); err != nil {
		return AdminCustomerDetail{}, err
	}
	return a.AdminCustomerDetail(ctx, item.ID)
}

func (a *ERPAuthenticator) supplierAllowedItems(ctx context.Context, principal Principal, query string, limit int) ([]SupplierItem, error) {
	state, err := a.adminSupplierState(principal.Ref)
	if err != nil {
		return nil, err
	}
	if state.Removed || state.Blocked {
		return []SupplierItem{}, nil
	}
	if !state.AssignmentsConfigured {
		items, err := a.erp.SearchSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, principal.Ref, query, limit)
		if err != nil {
			return nil, err
		}
		return a.mapSupplierItems(ctx, items)
	}
	if len(state.AssignedItemCodes) == 0 {
		return []SupplierItem{}, nil
	}

	items, err := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, state.AssignedItemCodes)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		items = filterItemsByQuery(items, trimmed)
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return a.mapSupplierItems(ctx, items)
}

func (a *ERPAuthenticator) validateSupplierItemAllowed(ctx context.Context, supplierRef, itemCode string) error {
	state, err := a.adminSupplierState(supplierRef)
	if err != nil {
		return err
	}
	if state.Removed || state.Blocked {
		return ErrInvalidCredentials
	}
	if !state.AssignmentsConfigured {
		return nil
	}
	if stateIncludesItem(state, itemCode) {
		return nil
	}
	return fmt.Errorf("item supplierga biriktirilmagan")
}

func (a *ERPAuthenticator) adminSupplierState(ref string) (AdminSupplierState, error) {
	if a.supplierAdmin == nil {
		return AdminSupplierState{}, nil
	}
	return a.supplierAdmin.Get(strings.TrimSpace(ref))
}

func (a *ERPAuthenticator) adminSupplierStates() (map[string]AdminSupplierState, error) {
	if a.supplierAdmin == nil {
		return map[string]AdminSupplierState{}, nil
	}
	return a.supplierAdmin.List()
}

func (a *ERPAuthenticator) saveAdminSupplierState(ref string, state AdminSupplierState) error {
	if a.supplierAdmin == nil {
		return nil
	}
	state.CustomCode = strings.TrimSpace(state.CustomCode)
	state.AssignedItemCodes = normalizeItemCodes(state.AssignedItemCodes)
	return a.supplierAdmin.Put(strings.TrimSpace(ref), state)
}

func (a *ERPAuthenticator) buildAdminSupplier(item erpnext.Supplier, state AdminSupplierState) (AdminSupplier, error) {
	code, err := a.supplierAccessCode(item, state)
	if err != nil {
		return AdminSupplier{}, err
	}
	return AdminSupplier{
		Ref:               item.ID,
		Name:              item.Name,
		Phone:             item.Phone,
		Code:              code,
		Blocked:           state.Blocked,
		Removed:           state.Removed,
		AssignedItemCodes: append([]string(nil), state.AssignedItemCodes...),
		AssignedItemCount: len(state.AssignedItemCodes),
	}, nil
}

func (a *ERPAuthenticator) supplierAccessCode(item erpnext.Supplier, state AdminSupplierState) (string, error) {
	if trimmed := strings.TrimSpace(state.CustomCode); trimmed != "" {
		return trimmed, nil
	}
	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   item.ID,
		Name:  item.Name,
		Phone: item.Phone,
	})
	if err != nil {
		return "", err
	}
	return creds.Code, nil
}

func (a *ERPAuthenticator) findSupplierForAdmin(ctx context.Context, ref string) (erpnext.Supplier, AdminSupplierState, error) {
	return a.findSupplierForAdminWithRemovedOption(ctx, ref, false)
}

func (a *ERPAuthenticator) findSupplierForAdminIncludingRemoved(ctx context.Context, ref string) (erpnext.Supplier, AdminSupplierState, error) {
	return a.findSupplierForAdminWithRemovedOption(ctx, ref, true)
}

func (a *ERPAuthenticator) findSupplierForAdminWithRemovedOption(ctx context.Context, ref string, includeRemoved bool) (erpnext.Supplier, AdminSupplierState, error) {
	doc, err := a.erp.GetSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, strings.TrimSpace(ref))
	if err != nil {
		return erpnext.Supplier{}, AdminSupplierState{}, err
	}
	if strings.TrimSpace(doc.ID) == "" {
		return erpnext.Supplier{}, AdminSupplierState{}, ErrAdminSupplierNotFound
	}

	state, err := a.adminSupplierState(doc.ID)
	if err != nil {
		return erpnext.Supplier{}, AdminSupplierState{}, err
	}
	if state.Removed && !includeRemoved {
		return erpnext.Supplier{}, AdminSupplierState{}, ErrAdminSupplierNotFound
	}
	return doc, state, nil
}

func (a *ERPAuthenticator) adminAssignedItems(ctx context.Context, supplierRef string, state AdminSupplierState, limit int) ([]SupplierItem, error) {
	items, err := a.erp.ListAssignedSupplierItems(ctx, a.baseURL, a.apiKey, a.apiSecret, supplierRef, limit)
	if err == nil {
		return a.mapSupplierItems(ctx, items)
	}
	if !isItemSupplierPermissionError(err) {
		return nil, err
	}

	if len(state.AssignedItemCodes) == 0 {
		return []SupplierItem{}, nil
	}

	fallbackItems, fallbackErr := a.erp.GetItemsByCodes(ctx, a.baseURL, a.apiKey, a.apiSecret, state.AssignedItemCodes)
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	return a.mapSupplierItems(ctx, fallbackItems)
}

func isItemSupplierPermissionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "permissionerror") || strings.Contains(message, "status 403:")
}

func (a *ERPAuthenticator) bumpCodeRegenState(state AdminSupplierState, now time.Time) (AdminSupplierState, error) {
	if state.isCodeLocked(now) {
		return state, ErrCodeRegenCooldown
	}

	if state.RegenWindowStartedAt.IsZero() || now.Sub(state.RegenWindowStartedAt) >= codeRegenWindow {
		state.RegenWindowStartedAt = now
		state.RegenWindowCount = 0
		state.CooldownUntil = time.Time{}
	}

	state.RegenWindowCount++
	if state.RegenWindowCount >= maxCodeRegensPerWindow {
		state.CooldownUntil = state.RegenWindowStartedAt.Add(codeRegenWindow)
	}
	return state, nil
}

func (state AdminSupplierState) isCodeLocked(now time.Time) bool {
	return !state.CooldownUntil.IsZero() && now.Before(state.CooldownUntil)
}

func (state AdminSupplierState) retryAfterSeconds(now time.Time) int {
	if !state.isCodeLocked(now) {
		return 0
	}
	seconds := int(state.CooldownUntil.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (a *ERPAuthenticator) scheduleSupplierCodePersist(ref, code string, dueAt time.Time) {
	go func() {
		wait := time.Until(dueAt)
		if wait > 0 {
			time.Sleep(wait)
		}

		state, err := a.adminSupplierState(ref)
		if err != nil {
			return
		}
		if strings.TrimSpace(state.PendingPersistCode) != strings.TrimSpace(code) {
			return
		}
		if time.Until(state.PendingPersistAt) > 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		supplier, err := a.erp.GetSupplier(ctx, a.baseURL, a.apiKey, a.apiSecret, ref)
		if err != nil {
			return
		}
		details := upsertAccordCodeInDetails(supplier.Details, code)
		if err := a.erp.UpdateSupplierDetails(ctx, a.baseURL, a.apiKey, a.apiSecret, ref, details); err != nil {
			return
		}

		state.PendingPersistCode = ""
		state.PendingPersistAt = time.Time{}
		_ = a.saveAdminSupplierState(ref, state)
	}()
}

func upsertAccordCodeInDetails(details, code string) string {
	lines := strings.Split(strings.ReplaceAll(details, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, accordCodeLinePrefix) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	filtered = append(filtered, accordCodeLinePrefix+" "+strings.TrimSpace(code))
	return strings.Join(filtered, "\n")
}

func upsertSupplierPhoneInDetails(details, phone string) string {
	lines := strings.Split(strings.ReplaceAll(details, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "telefon:") || strings.HasPrefix(lower, "phone:") {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	filtered = append([]string{"Telefon: " + strings.TrimSpace(phone)}, filtered...)
	return strings.Join(filtered, "\n")
}

func (a *ERPAuthenticator) mapSupplierItems(ctx context.Context, items []erpnext.Item) ([]SupplierItem, error) {
	warehouse, err := a.resolveWarehouse(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SupplierItem, 0, len(items))
	for _, item := range items {
		result = append(result, SupplierItem{
			Code:      item.Code,
			Name:      item.Name,
			UOM:       item.UOM,
			Warehouse: warehouse,
		})
	}
	return result, nil
}

func normalizeItemCodes(itemCodes []string) []string {
	normalized := make([]string, 0, len(itemCodes))
	seen := make(map[string]struct{}, len(itemCodes))
	for _, code := range itemCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func filterItemsByQuery(items []erpnext.Item, query string) []erpnext.Item {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return items
	}

	filtered := make([]erpnext.Item, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Code), lowerQuery) ||
			strings.Contains(strings.ToLower(item.Name), lowerQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func randomSupplierCode(prefix string, existing map[string]struct{}) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		prefix = "10"
	}
	for attempts := 0; attempts < 64; attempts++ {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		builder := strings.Builder{}
		builder.Grow(len(prefix) + len(buf))
		builder.WriteString(prefix)
		for _, value := range buf {
			builder.WriteByte(supplierCodeAlphabet[int(value)%len(supplierCodeAlphabet)])
		}
		code := builder.String()
		if _, ok := existing[code]; ok {
			continue
		}
		return code, nil
	}
	return "", fmt.Errorf("supplier code generation failed")
}

func stateIncludesItem(state AdminSupplierState, itemCode string) bool {
	return slices.ContainsFunc(state.AssignedItemCodes, func(candidate string) bool {
		return strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(itemCode))
	})
}

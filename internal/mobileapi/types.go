package mobileapi

import "mobile_server/internal/core"

type PrincipalRole = core.PrincipalRole

const (
	RoleSupplier = core.RoleSupplier
	RoleWerka    = core.RoleWerka
	RoleCustomer = core.RoleCustomer
	RoleAdmin    = core.RoleAdmin
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type LoginRequest = core.LoginRequest
type LoginResponse = core.LoginResponse
type DispatchRecord = core.DispatchRecord
type NotificationComment = core.NotificationComment
type NotificationDetail = core.NotificationDetail
type SupplierItem = core.SupplierItem
type SupplierDirectoryEntry = core.SupplierDirectoryEntry
type CustomerDirectoryEntry = core.CustomerDirectoryEntry
type SupplierHomeSummary = core.SupplierHomeSummary
type SupplierStatusBreakdownEntry = core.SupplierStatusBreakdownEntry
type WerkaHomeSummary = core.WerkaHomeSummary
type WerkaStatusBreakdownEntry = core.WerkaStatusBreakdownEntry
type CreateDispatchRequest = core.CreateDispatchRequest
type ConfirmReceiptRequest = core.ConfirmReceiptRequest
type WerkaUnannouncedCreateRequest = core.WerkaUnannouncedCreateRequest
type WerkaCustomerIssueCreateRequest = core.WerkaCustomerIssueCreateRequest
type WerkaCustomerIssueRecord = core.WerkaCustomerIssueRecord
type SupplierUnannouncedResponseRequest = core.SupplierUnannouncedResponseRequest
type NotificationCommentCreateRequest = core.NotificationCommentCreateRequest
type PushTokenRegisterRequest = core.PushTokenRegisterRequest
type ProfileUpdateRequest = core.ProfileUpdateRequest
type AdminSettings = core.AdminSettings
type AdminSupplier = core.AdminSupplier
type AdminCreateSupplierRequest = core.AdminCreateSupplierRequest
type AdminCreateCustomerRequest = core.AdminCreateCustomerRequest
type AdminSupplierSummary = core.AdminSupplierSummary
type AdminSupplierDetail = core.AdminSupplierDetail
type AdminCustomerDetail = core.AdminCustomerDetail
type AdminCustomerPhoneUpdateRequest = core.AdminCustomerPhoneUpdateRequest
type AdminSupplierStatusUpdateRequest = core.AdminSupplierStatusUpdateRequest
type AdminSupplierPhoneUpdateRequest = core.AdminSupplierPhoneUpdateRequest
type AdminSupplierItemsUpdateRequest = core.AdminSupplierItemsUpdateRequest
type AdminSupplierItemMutationRequest = core.AdminSupplierItemMutationRequest
type AdminCreateItemRequest = core.AdminCreateItemRequest

package mobileapi

import "mobile_server/internal/core"

type PrincipalRole = core.PrincipalRole

const (
	RoleSupplier = core.RoleSupplier
	RoleWerka    = core.RoleWerka
)

type Principal = core.Principal
type Authenticator = core.Authenticator
type LoginRequest = core.LoginRequest
type LoginResponse = core.LoginResponse
type DispatchRecord = core.DispatchRecord
type SupplierItem = core.SupplierItem
type CreateDispatchRequest = core.CreateDispatchRequest
type ConfirmReceiptRequest = core.ConfirmReceiptRequest
type ProfileUpdateRequest = core.ProfileUpdateRequest

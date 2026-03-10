package mobileapi

import "mobile_server/internal/core"

var (
	ErrInvalidCredentials = core.ErrInvalidCredentials
	ErrInvalidRole        = core.ErrInvalidRole
	ErrUnauthorized       = core.ErrUnauthorized
)

type ERPClient = core.ERPClient
type ERPAuthenticator = core.ERPAuthenticator
type SessionManager = core.SessionManager

func NewERPAuthenticator(
	erp ERPClient,
	baseURL string,
	apiKey string,
	apiSecret string,
	defaultWarehouse string,
	supplierPrefix string,
	werkaPrefix string,
	werkaCode string,
	werkaPhone string,
	werkaName string,
	profiles *ProfileStore,
) *ERPAuthenticator {
	return core.NewERPAuthenticator(
		erp,
		baseURL,
		apiKey,
		apiSecret,
		defaultWarehouse,
		supplierPrefix,
		werkaPrefix,
		werkaCode,
		werkaPhone,
		werkaName,
		profiles,
	)
}

func NewSessionManager() *SessionManager {
	return core.NewSessionManager()
}

func requireRole(principal Principal, role PrincipalRole) error {
	return core.RequireRole(principal, role)
}

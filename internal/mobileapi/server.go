package mobileapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	auth     *ERPAuthenticator
	sessions *SessionManager
	push     *PushTokenStore
	sender   pushSender
	aiSearch *werkaAISearchService
}

func NewServer(auth *ERPAuthenticator) *Server {
	return NewServerWithSessionManager(auth, nil)
}

func NewServerWithSessionManager(auth *ERPAuthenticator, sessions *SessionManager) *Server {
	pushStore := NewPushTokenStore("data/mobile_push_tokens.json")
	if sessions == nil {
		sessions = NewSessionManager()
	}
	return &Server{
		auth:     auth,
		sessions: sessions,
		push:     pushStore,
		sender:   newPushSender(pushStore),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/mobile/auth/login", s.handleLogin)
	mux.HandleFunc("/v1/mobile/auth/logout", s.handleLogout)
	mux.HandleFunc("/v1/mobile/me", s.handleMe)
	mux.HandleFunc("/v1/mobile/profile", s.handleProfile)
	mux.HandleFunc("/v1/mobile/profile/avatar", s.handleProfileAvatar)
	mux.HandleFunc("/v1/mobile/profile/avatar/view", s.handleProfileAvatarView)
	mux.HandleFunc("/v1/mobile/push/token", s.handlePushToken)
	mux.HandleFunc("/v1/mobile/customer/summary", s.handleCustomerSummary)
	mux.HandleFunc("/v1/mobile/customer/history", s.handleCustomerHistory)
	mux.HandleFunc("/v1/mobile/customer/status-details", s.handleCustomerStatusDetails)
	mux.HandleFunc("/v1/mobile/customer/detail", s.handleCustomerDetail)
	mux.HandleFunc("/v1/mobile/customer/respond", s.handleCustomerRespond)
	mux.HandleFunc("/v1/mobile/notifications/detail", s.handleNotificationDetail)
	mux.HandleFunc("/v1/mobile/notifications/comments", s.handleNotificationComment)
	mux.HandleFunc("/v1/mobile/supplier/unannounced/respond", s.handleSupplierUnannouncedRespond)
	mux.HandleFunc("/v1/mobile/supplier/summary", s.handleSupplierSummary)
	mux.HandleFunc("/v1/mobile/supplier/status-breakdown", s.handleSupplierStatusBreakdown)
	mux.HandleFunc("/v1/mobile/supplier/status-details", s.handleSupplierStatusDetails)
	mux.HandleFunc("/v1/mobile/supplier/history", s.handleSupplierHistory)
	mux.HandleFunc("/v1/mobile/supplier/items", s.handleSupplierItems)
	mux.HandleFunc("/v1/mobile/supplier/dispatch", s.handleCreateDispatch)
	mux.HandleFunc("/v1/mobile/werka/summary", s.handleWerkaSummary)
	mux.HandleFunc("/v1/mobile/werka/home", s.handleWerkaHome)
	mux.HandleFunc("/v1/mobile/werka/customers", s.handleWerkaCustomers)
	mux.HandleFunc("/v1/mobile/werka/suppliers", s.handleWerkaSuppliers)
	mux.HandleFunc("/v1/mobile/werka/ai-search-suggestion", s.handleWerkaAISearchSuggestion)
	mux.HandleFunc("/v1/mobile/werka/supplier-items", s.handleWerkaSupplierItems)
	mux.HandleFunc("/v1/mobile/werka/customer-items", s.handleWerkaCustomerItems)
	mux.HandleFunc("/v1/mobile/werka/customer-item-options", s.handleWerkaCustomerItemOptions)
	mux.HandleFunc("/v1/mobile/werka/customer-issue/create", s.handleWerkaCustomerIssueCreate)
	mux.HandleFunc("/v1/mobile/werka/customer-issue/batch-create", s.handleWerkaCustomerIssueBatchCreate)
	mux.HandleFunc("/v1/mobile/werka/unannounced/create", s.handleWerkaUnannouncedCreate)
	mux.HandleFunc("/v1/mobile/werka/status-breakdown", s.handleWerkaStatusBreakdown)
	mux.HandleFunc("/v1/mobile/werka/status-details", s.handleWerkaStatusDetails)
	mux.HandleFunc("/v1/mobile/werka/pending", s.handleWerkaPending)
	mux.HandleFunc("/v1/mobile/werka/history", s.handleWerkaHistory)
	mux.HandleFunc("/v1/mobile/werka/confirm", s.handleWerkaConfirm)
	mux.HandleFunc("/v1/mobile/admin/settings", s.handleAdminSettings)
	mux.HandleFunc("/v1/mobile/admin/suppliers", s.handleAdminSuppliers)
	mux.HandleFunc("/v1/mobile/admin/customers", s.handleAdminCustomers)
	mux.HandleFunc("/v1/mobile/admin/customers/detail", s.handleAdminCustomerDetail)
	mux.HandleFunc("/v1/mobile/admin/customers/phone", s.handleAdminCustomerPhone)
	mux.HandleFunc("/v1/mobile/admin/customers/code/regenerate", s.handleAdminCustomerCodeRegenerate)
	mux.HandleFunc("/v1/mobile/admin/customers/items/add", s.handleAdminCustomerItemAdd)
	mux.HandleFunc("/v1/mobile/admin/customers/items/remove", s.handleAdminCustomerItemRemove)
	mux.HandleFunc("/v1/mobile/admin/customers/remove", s.handleAdminCustomerRemove)
	mux.HandleFunc("/v1/mobile/admin/suppliers/summary", s.handleAdminSupplierSummary)
	mux.HandleFunc("/v1/mobile/admin/suppliers/detail", s.handleAdminSupplierDetail)
	mux.HandleFunc("/v1/mobile/admin/suppliers/inactive", s.handleAdminInactiveSuppliers)
	mux.HandleFunc("/v1/mobile/admin/suppliers/status", s.handleAdminSupplierStatus)
	mux.HandleFunc("/v1/mobile/admin/suppliers/phone", s.handleAdminSupplierPhone)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items", s.handleAdminSupplierItems)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/assigned", s.handleAdminSupplierAssignedItems)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/add", s.handleAdminSupplierItemAdd)
	mux.HandleFunc("/v1/mobile/admin/suppliers/items/remove", s.handleAdminSupplierItemRemove)
	mux.HandleFunc("/v1/mobile/admin/suppliers/code/regenerate", s.handleAdminSupplierCodeRegenerate)
	mux.HandleFunc("/v1/mobile/admin/suppliers/remove", s.handleAdminSupplierRemove)
	mux.HandleFunc("/v1/mobile/admin/suppliers/restore", s.handleAdminSupplierRestore)
	mux.HandleFunc("/v1/mobile/admin/items", s.handleAdminItems)
	mux.HandleFunc("/v1/mobile/admin/activity", s.handleAdminActivity)
	mux.HandleFunc("/v1/mobile/admin/werka/code/regenerate", s.handleAdminWerkaCodeRegenerate)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func avatarProxyURL(r *http.Request, principal Principal, token string) string {
	if principal.Role != RoleSupplier || strings.TrimSpace(principal.Ref) == "" {
		return principal.AvatarURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/v1/mobile/profile/avatar/view?token=%s", scheme, r.Host, url.QueryEscape(strings.TrimSpace(token)))
}

func withAvatarProxy(r *http.Request, principal Principal, token string) Principal {
	if strings.TrimSpace(principal.AvatarURL) == "" {
		return principal
	}
	principal.AvatarURL = avatarProxyURL(r, principal, token)
	return principal
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	principal, err := s.auth.Login(r.Context(), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Code))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidRole):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	if current, err := s.auth.Profile(r.Context(), principal); err == nil {
		principal = current
	}

	token, err := s.sessions.Create(principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session create failed"})
		return
	}

	var werkaHome *WerkaHomeData
	if principal.Role == RoleWerka {
		if data, err := s.auth.WerkaHome(r.Context(), 20); err == nil {
			werkaHome = &data
		}
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		Profile:   withAvatarProxy(r, principal, token),
		WerkaHome: werkaHome,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}
	if current, err := s.auth.Profile(r.Context(), principal); err == nil {
		principal = current
		s.sessions.Update(token, principal)
	}
	writeJSON(w, http.StatusOK, withAvatarProxy(r, principal, token))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	token, err := bearerToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.sessions.Delete(token)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		current, err := s.auth.Profile(r.Context(), principal)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile fetch failed"})
			return
		}
		s.sessions.Update(token, current)
		writeJSON(w, http.StatusOK, withAvatarProxy(r, current, token))
	case http.MethodPut:
		var req ProfileUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		current, err := s.auth.UpdateNickname(principal, req.Nickname)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "nickname update failed"})
			return
		}
		s.sessions.Update(token, current)
		writeJSON(w, http.StatusOK, withAvatarProxy(r, current, token))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleProfileAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	token, principal, ok := s.authorizeWithToken(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart"})
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar read failed"})
		return
	}

	current, err := s.auth.UploadAvatar(
		r.Context(),
		principal,
		header.Filename,
		header.Header.Get("Content-Type"),
		content,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "avatar upload failed"})
		return
	}

	s.sessions.Update(token, current)
	writeJSON(w, http.StatusOK, withAvatarProxy(r, current, token))
}

func (s *Server) handleProfileAvatarView(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	var (
		principal Principal
		ok        bool
	)
	if token != "" {
		principal, ok = s.sessions.Get(token)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
	} else {
		principal, ok = s.authorize(w, r)
		if !ok {
			return
		}
	}
	if principal.Role != RoleSupplier {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	current, err := s.auth.Profile(r.Context(), principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "avatar fetch failed"})
		return
	}
	if strings.TrimSpace(current.AvatarURL) == "" {
		http.NotFound(w, r)
		return
	}
	contentType, body, err := s.auth.DownloadFile(
		r.Context(),
		s.auth.BaseURL(),
		s.auth.APIKey(),
		s.auth.APISecret(),
		current.AvatarURL,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "avatar fetch failed"})
		return
	}
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleNotificationDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if principal.Role != RoleSupplier && principal.Role != RoleWerka && principal.Role != RoleCustomer {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	receiptID := strings.TrimSpace(r.URL.Query().Get("receipt_id"))
	if receiptID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "receipt_id is required"})
		return
	}

	detail, err := s.auth.NotificationDetail(r.Context(), principal, receiptID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification detail failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleNotificationComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if principal.Role != RoleSupplier &&
		principal.Role != RoleWerka &&
		principal.Role != RoleCustomer {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	receiptID := strings.TrimSpace(r.URL.Query().Get("receipt_id"))
	if receiptID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "receipt_id is required"})
		return
	}
	var req NotificationCommentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AddNotificationComment(r.Context(), principal, receiptID, req.Message)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification comment failed"})
		return
	}
	if principal.Role == RoleSupplier && strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Message)), "tasdiqlayman") {
		record := detail.Record
		record.ID = "supplier_ack:" + strings.TrimSpace(record.ID) + ":" + fmt.Sprintf("%d", time.Now().Unix())
		record.EventType = "supplier_ack"
		record.Highlight = "Supplier mahsulotni qaytarganingizni tasdiqladi"
		record.Note = ""
		if err := s.sender.SendToKey(
			r.Context(),
			string(RoleWerka)+":werka",
			"Supplier tasdiqladi",
			record.Highlight,
			dispatchRecordDataForTarget(record, RoleWerka, "werka"),
		); err != nil {
			log.Printf("push send failed for werka acknowledgment event: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handlePushToken(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if principal.Role != RoleSupplier && principal.Role != RoleWerka {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req PushTokenRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(req.Token) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
			return
		}
		key := pushTokenKey(principal)
		before, err := s.push.List(key)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token read failed"})
			return
		}
		log.Printf(
			"push token register requested key=%s token=%s platform=%s existing_count=%d",
			key,
			truncateToken(req.Token),
			strings.TrimSpace(req.Platform),
			len(before),
		)
		if err := s.push.MoveTokenToKey(key, req.Token, req.Platform); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token save failed"})
			return
		}
		after, err := s.push.List(key)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token read failed"})
			return
		}
		log.Printf(
			"push token register stored key=%s token=%s total_count=%d",
			key,
			truncateToken(req.Token),
			len(after),
		)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
			return
		}
		key := pushTokenKey(principal)
		before, err := s.push.List(key)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token read failed"})
			return
		}
		log.Printf(
			"push token delete requested key=%s token=%s existing_count=%d",
			key,
			truncateToken(token),
			len(before),
		)
		if err := s.push.Delete(key, token); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token delete failed"})
			return
		}
		after, err := s.push.List(key)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "push token read failed"})
			return
		}
		log.Printf(
			"push token delete stored key=%s token=%s total_count=%d",
			key,
			truncateToken(token),
			len(after),
		)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func pushTokenKey(principal Principal) string {
	return string(principal.Role) + ":" + strings.TrimSpace(principal.Ref)
}

func dispatchRecordData(record DispatchRecord) map[string]string {
	return map[string]string{
		"id":            record.ID,
		"record_type":   record.RecordType,
		"supplier_ref":  record.SupplierRef,
		"supplier_name": record.SupplierName,
		"item_code":     record.ItemCode,
		"item_name":     record.ItemName,
		"uom":           record.UOM,
		"sent_qty":      fmt.Sprintf("%.4f", record.SentQty),
		"accepted_qty":  fmt.Sprintf("%.4f", record.AcceptedQty),
		"amount":        fmt.Sprintf("%.4f", record.Amount),
		"currency":      record.Currency,
		"note":          record.Note,
		"event_type":    record.EventType,
		"highlight":     record.Highlight,
		"status":        string(record.Status),
		"created_label": record.CreatedLabel,
	}
}

func dispatchRecordDataForTarget(record DispatchRecord, role PrincipalRole, ref string) map[string]string {
	data := dispatchRecordData(record)
	data["target_role"] = string(role)
	data["target_ref"] = strings.TrimSpace(ref)
	return data
}

func (s *Server) handleSupplierHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.SupplierHistory(r.Context(), principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier history failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSupplierSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	summary, err := s.auth.SupplierSummary(r.Context(), principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier summary failed"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCustomerSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleCustomer); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	summary, err := s.auth.CustomerSummary(r.Context(), principal)
	if err != nil {
		log.Printf("customer summary failed for ref=%s err=%v", strings.TrimSpace(principal.Ref), err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer summary failed"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCustomerHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleCustomer); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.CustomerHistory(r.Context(), principal)
	if err != nil {
		log.Printf("customer history failed for ref=%s err=%v", strings.TrimSpace(principal.Ref), err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer history failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCustomerStatusDetails(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleCustomer); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	items, err := s.auth.CustomerStatusDetails(r.Context(), principal, kind)
	if err != nil {
		log.Printf("customer status details failed for ref=%s kind=%s err=%v", strings.TrimSpace(principal.Ref), kind, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer status details failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCustomerDetail(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleCustomer); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	deliveryNoteID := strings.TrimSpace(r.URL.Query().Get("delivery_note_id"))
	if deliveryNoteID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "delivery_note_id is required"})
		return
	}

	detail, err := s.auth.CustomerDeliveryDetail(r.Context(), principal, deliveryNoteID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer detail failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleCustomerRespond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleCustomer); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req CustomerDeliveryResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.CustomerRespondDeliveryRequest(
		r.Context(),
		principal,
		req,
	)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer respond failed"})
		return
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleWerka)+":werka",
		"Customer javob berdi",
		detail.Record.Note,
		dispatchRecordDataForTarget(detail.Record, RoleWerka, "werka"),
	); err != nil {
		log.Printf("push send failed for werka customer response: %v", err)
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleAdmin)+":admin",
		"Customer javob berdi",
		detail.Record.Note,
		dispatchRecordDataForTarget(detail.Record, RoleAdmin, "admin"),
	); err != nil {
		log.Printf("push send failed for admin customer response: %v", err)
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleSupplierStatusBreakdown(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	items, err := s.auth.SupplierStatusBreakdown(r.Context(), principal, kind)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier status breakdown failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSupplierStatusDetails(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	items, err := s.auth.SupplierStatusDetails(r.Context(), principal, kind, itemCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier status details failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSupplierItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := s.auth.SupplierItems(r.Context(), principal, query, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier items failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSupplierUnannouncedRespond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req SupplierUnannouncedResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	detail, err := s.auth.RespondWerkaUnannouncedDraft(r.Context(), principal, req.ReceiptID, req.Approve, req.Reason)
	if err != nil {
		log.Printf("supplier unannounced response failed for %s approve=%v reason=%q: %v", req.ReceiptID, req.Approve, req.Reason, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier unannounced response failed"})
		return
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleWerka)+":werka",
		"Supplier javob berdi",
		detail.Record.Note,
		dispatchRecordDataForTarget(detail.Record, RoleWerka, "werka"),
	); err != nil {
		log.Printf("push send failed for werka unannounced response: %v", err)
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleCreateDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleSupplier); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req CreateDispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	record, err := s.auth.CreateDispatch(r.Context(), principal, req.ItemCode, req.Qty)
	if err != nil {
		log.Printf("supplier dispatch create failed for %s/%s: %v", principal.Ref, req.ItemCode, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dispatch create failed"})
		return
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleWerka)+":werka",
		record.SupplierName,
		fmt.Sprintf("%s • %.0f %s qabul kutmoqda.", record.ItemCode, record.SentQty, record.UOM),
		dispatchRecordDataForTarget(record, RoleWerka, "werka"),
	); err != nil {
		log.Printf("push send failed for werka dispatch notify: %v", err)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleWerkaPending(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.WerkaPending(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pending fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	summary, err := s.auth.WerkaSummary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka summary failed"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleWerkaHome(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	data, err := s.auth.WerkaHome(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka home failed"})
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleWerkaSuppliers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := optionalSearchLimit(r, 200, 200)
	offset := optionalSearchOffset(r)
	items, err := s.auth.WerkaSuppliersPage(r.Context(), query, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka suppliers failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaCustomers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := optionalSearchLimit(r, 200, 200)
	offset := optionalSearchOffset(r)
	items, err := s.auth.WerkaCustomersPage(r.Context(), query, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka customers failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaSupplierItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	supplierRef := strings.TrimSpace(r.URL.Query().Get("supplier_ref"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := optionalSearchLimit(r, 100, 200)
	offset := optionalSearchOffset(r)
	items, err := s.auth.WerkaSupplierItemsPage(r.Context(), supplierRef, query, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka supplier items failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaCustomerItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	customerRef := strings.TrimSpace(r.URL.Query().Get("customer_ref"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := optionalSearchLimit(r, 100, 200)
	offset := optionalSearchOffset(r)
	items, err := s.auth.WerkaCustomerItemsPage(r.Context(), customerRef, query, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka customer items failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaCustomerItemOptions(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := optionalSearchLimit(r, 200, 200)
	offset := optionalSearchOffset(r)
	items, err := s.auth.WerkaCustomerItemOptionsPage(r.Context(), query, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka customer item options failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func optionalSearchLimit(r *http.Request, defaultLimit, maxLimit int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultLimit
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultLimit
	}
	if maxLimit > 0 && value > maxLimit {
		return maxLimit
	}
	return value
}

func optionalSearchOffset(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("offset"))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func (s *Server) handleWerkaCustomerIssueCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req WerkaCustomerIssueCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	log.Printf(
		"werka customer issue create requested by=%s customer=%s item=%s qty=%.4f",
		strings.TrimSpace(principal.Ref),
		strings.TrimSpace(req.CustomerRef),
		strings.TrimSpace(req.ItemCode),
		req.Qty,
	)
	record, err := s.auth.CreateWerkaCustomerIssue(r.Context(), principal, req.CustomerRef, req.ItemCode, req.Qty)
	if err != nil {
		log.Printf(
			"werka customer issue create failed by=%s customer=%s item=%s qty=%.4f err=%v",
			strings.TrimSpace(principal.Ref),
			strings.TrimSpace(req.CustomerRef),
			strings.TrimSpace(req.ItemCode),
			req.Qty,
			err,
		)
		if errors.Is(err, ErrInsufficientStock) {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error":      "insufficient stock",
				"error_code": "insufficient_stock",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka customer issue create failed"})
		return
	}
	log.Printf(
		"werka customer issue create succeeded by=%s customer=%s item=%s qty=%.4f delivery_note=%s",
		strings.TrimSpace(principal.Ref),
		strings.TrimSpace(record.CustomerRef),
		strings.TrimSpace(record.ItemCode),
		record.Qty,
		strings.TrimSpace(record.EntryID),
	)
	if err := s.sendWerkaCustomerIssuePush(r.Context(), record); err != nil {
		log.Printf("push send failed for customer delivery note: %v", err)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleWerkaCustomerIssueBatchCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req WerkaCustomerIssueBatchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.Lines) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lines are required"})
		return
	}

	results := make([]WerkaCustomerIssueBatchLineResult, len(req.Lines))
	workerCount := len(req.Lines)
	if workerCount > 4 {
		workerCount = 4
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				line := req.Lines[index]
				record, err := s.auth.CreateWerkaCustomerIssue(
					r.Context(),
					principal,
					line.CustomerRef,
					line.ItemCode,
					line.Qty,
				)
				if err != nil {
					result := WerkaCustomerIssueBatchLineResult{
						LineIndex: index,
						Error:     "werka customer issue create failed",
					}
					if errors.Is(err, ErrInsufficientStock) {
						result.Error = "insufficient stock"
						result.ErrorCode = "insufficient_stock"
					}
					results[index] = result
					continue
				}
				recordCopy := record
				results[index] = WerkaCustomerIssueBatchLineResult{
					LineIndex: index,
					Record:    &recordCopy,
				}
			}
		}()
	}
	for index := range req.Lines {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	response := WerkaCustomerIssueBatchResult{
		ClientBatchID: strings.TrimSpace(req.ClientBatchID),
		Created:       make([]WerkaCustomerIssueBatchLineResult, 0, len(req.Lines)),
		Failed:        make([]WerkaCustomerIssueBatchLineResult, 0, len(req.Lines)),
	}
	for _, result := range results {
		if result.Record != nil {
			log.Printf(
				"werka customer issue batch created by=%s line=%d customer=%s item=%s qty=%.4f delivery_note=%s",
				strings.TrimSpace(principal.Ref),
				result.LineIndex,
				strings.TrimSpace(result.Record.CustomerRef),
				strings.TrimSpace(result.Record.ItemCode),
				result.Record.Qty,
				strings.TrimSpace(result.Record.EntryID),
			)
			if err := s.sendWerkaCustomerIssuePush(r.Context(), *result.Record); err != nil {
				log.Printf("push send failed for customer delivery note batch line=%d: %v", result.LineIndex, err)
			}
			response.Created = append(response.Created, result)
			continue
		}
		log.Printf(
			"werka customer issue batch failed by=%s line=%d err=%s code=%s",
			strings.TrimSpace(principal.Ref),
			result.LineIndex,
			strings.TrimSpace(result.Error),
			strings.TrimSpace(result.ErrorCode),
		)
		response.Failed = append(response.Failed, result)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) sendWerkaCustomerIssuePush(ctx context.Context, record WerkaCustomerIssueRecord) error {
	return s.sender.SendToKey(
		ctx,
		string(RoleCustomer)+":"+strings.TrimSpace(record.CustomerRef),
		"Werka mahsulot jo'natdi",
		fmt.Sprintf("%s %.0f %s jo'natildi", strings.TrimSpace(record.ItemCode), record.Qty, strings.TrimSpace(record.UOM)),
		dispatchRecordDataForTarget(
			DispatchRecord{
				ID:           record.EntryID,
				SupplierRef:  record.CustomerRef,
				SupplierName: record.CustomerName,
				ItemCode:     record.ItemCode,
				ItemName:     record.ItemName,
				UOM:          record.UOM,
				SentQty:      record.Qty,
				AcceptedQty:  0,
				Status:       "pending",
				CreatedLabel: record.CreatedLabel,
			},
			RoleCustomer,
			record.CustomerRef,
		),
	)
}

func (s *Server) handleWerkaUnannouncedCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req WerkaUnannouncedCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	record, err := s.auth.CreateWerkaUnannouncedDraft(r.Context(), principal, req.SupplierRef, req.ItemCode, req.Qty)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka unannounced create failed"})
		return
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleSupplier)+":"+strings.TrimSpace(record.SupplierRef),
		"Werka siz qayd etmagan mahsulotni qabul qildi",
		"Tasdiqlash kutilmoqda",
		dispatchRecordDataForTarget(record, RoleSupplier, record.SupplierRef),
	); err != nil {
		log.Printf("push send failed for supplier unannounced draft: %v", err)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleWerkaStatusBreakdown(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	items, err := s.auth.WerkaStatusBreakdown(r.Context(), kind)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka status breakdown failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaStatusDetails(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	supplierRef := strings.TrimSpace(r.URL.Query().Get("supplier_ref"))
	items, err := s.auth.WerkaStatusDetails(r.Context(), kind, supplierRef)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka status details failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaHistory(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	items, err := s.auth.WerkaHistory(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWerkaConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleWerka); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req ConfirmReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	record, err := s.auth.ConfirmReceipt(
		r.Context(),
		req.ReceiptID,
		req.AcceptedQty,
		req.ReturnedQty,
		req.ReturnReason,
		req.ReturnComment,
	)
	if err != nil {
		log.Printf("werka confirm failed for %s accepted=%.4f returned=%.4f reason=%q comment=%q: %v", req.ReceiptID, req.AcceptedQty, req.ReturnedQty, req.ReturnReason, req.ReturnComment, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "receipt confirm failed"})
		return
	}
	if err := s.sender.SendToKey(
		r.Context(),
		string(RoleSupplier)+":"+strings.TrimSpace(record.SupplierRef),
		record.ItemCode,
		fmt.Sprintf("Status: %s", strings.TrimSpace(record.Status)),
		dispatchRecordDataForTarget(record, RoleSupplier, record.SupplierRef),
	); err != nil {
		log.Printf("push send failed for supplier receipt notify (%s): %v", strings.TrimSpace(record.SupplierName), err)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.auth.AdminSettings())
	case http.MethodPut:
		var req AdminSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := s.auth.UpdateAdminSettings(req); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings update failed"})
			return
		}
		writeJSON(w, http.StatusOK, s.auth.AdminSettings())
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAdminSuppliers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	items, err := s.auth.AdminSuppliers(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "suppliers fetch failed"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, items)
		return
	}
	if r.Method == http.MethodPost {
		var req AdminCreateSupplierRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		item, err := s.auth.AdminCreateSupplier(r.Context(), req.Name, req.Phone)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier create failed"})
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (s *Server) handleAdminCustomers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method == http.MethodGet {
		items, err := s.auth.AdminCustomers(r.Context(), 500)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customers fetch failed"})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req AdminCreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	item, err := s.auth.AdminCreateCustomer(r.Context(), req.Name, req.Phone)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer create failed"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAdminCustomerDetail(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	detail, err := s.auth.AdminCustomerDetail(r.Context(), ref)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer detail failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminCustomerPhone(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminCustomerPhoneUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	detail, err := s.auth.AdminUpdateCustomerPhone(r.Context(), ref, req.Phone)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer phone update failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminCustomerCodeRegenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	detail, err := s.auth.AdminRegenerateCustomerCode(r.Context(), ref)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer code regenerate failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminCustomerRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	if err := s.auth.AdminRemoveCustomer(r.Context(), ref); err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminCustomerItemAdd(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierItemMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	detail, err := s.auth.AdminAssignCustomerItem(r.Context(), ref, req.ItemCode)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer item add failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminCustomerItemRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if ref == "" || itemCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref and item_code are required"})
		return
	}
	detail, err := s.auth.AdminUnassignCustomerItem(r.Context(), ref, itemCode)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "customer item remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierSummary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	summary, err := s.auth.AdminSupplierSummary(r.Context(), 300)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier summary failed"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleAdminSupplierDetail(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	detail, err := s.auth.AdminSupplierDetail(r.Context(), ref)
	if err != nil {
		log.Printf("admin supplier detail failed for %s: %v", ref, err)
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier detail failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminInactiveSuppliers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	items, err := s.auth.AdminInactiveSuppliers(r.Context(), 300)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "inactive suppliers failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminSupplierStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierStatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AdminSetSupplierBlocked(r.Context(), ref, req.Blocked)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier status failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierPhone(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierPhoneUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AdminUpdateSupplierPhone(r.Context(), ref, req.Phone)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier phone update failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierItemsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	detail, err := s.auth.AdminUpdateSupplierItems(r.Context(), ref, req.ItemCodes)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier items update failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierAssignedItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	items, err := s.auth.AdminAssignedSupplierItems(r.Context(), ref, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "assigned items fetch failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminSupplierItemAdd(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}
	var req AdminSupplierItemMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	detail, err := s.auth.AdminAssignSupplierItem(r.Context(), ref, req.ItemCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier item add failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierItemRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if ref == "" || itemCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref and item_code are required"})
		return
	}
	detail, err := s.auth.AdminUnassignSupplierItem(r.Context(), ref, itemCode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier item remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierCodeRegenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	detail, err := s.auth.AdminRegenerateSupplierCode(r.Context(), ref)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		if errors.Is(err, ErrCodeRegenCooldown) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code regenerate cooldown"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier code regenerate failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminSupplierRemove(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	if err := s.auth.AdminRemoveSupplier(r.Context(), ref); err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier remove failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminSupplierRestore(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ref is required"})
		return
	}

	detail, err := s.auth.AdminRestoreSupplier(r.Context(), ref)
	if err != nil {
		if errors.Is(err, ErrAdminSupplierNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "supplier not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "supplier restore failed"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAdminItems(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		items, err := s.auth.AdminSearchItems(r.Context(), query, 30)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin items failed"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req AdminCreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		item, err := s.auth.AdminCreateItem(r.Context(), req.Code, req.Name, req.UOM)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin item create failed"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	items, err := s.auth.AdminActivity(r.Context(), 30)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "admin activity failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminWerkaCodeRegenerate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if err := requireRole(principal, RoleAdmin); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	settings, err := s.auth.AdminRegenerateWerkaCode()
	if err != nil {
		if errors.Is(err, ErrCodeRegenCooldown) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code regenerate cooldown"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "werka code regenerate failed"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	_, principal, ok := s.authorizeWithToken(w, r)
	return principal, ok
}

func (s *Server) authorizeWithToken(w http.ResponseWriter, r *http.Request) (string, Principal, bool) {
	token, err := bearerToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return "", Principal{}, false
	}

	principal, ok := s.sessions.Get(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return "", Principal{}, false
	}
	return token, principal, true
}

func bearerToken(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return "", ErrUnauthorized
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

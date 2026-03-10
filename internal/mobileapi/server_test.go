package mobileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"mobile_server/internal/erpnext"
	"mobile_server/internal/suplier"
)

type fakeERPClient struct {
	suppliers         []erpnext.Supplier
	uploadedAvatarURL string
}

func (f *fakeERPClient) SearchSuppliers(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.Supplier, error) {
	return f.suppliers, nil
}

func (f *fakeERPClient) SearchSupplierItems(_ context.Context, _, _, _, _, _ string, _ int) ([]erpnext.Item, error) {
	return nil, nil
}

func (f *fakeERPClient) GetSupplier(_ context.Context, _, _, _, id string) (erpnext.Supplier, error) {
	for _, item := range f.suppliers {
		if item.ID == id {
			return item, nil
		}
	}
	return erpnext.Supplier{}, nil
}

func (f *fakeERPClient) ListPendingPurchaseReceipts(_ context.Context, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (f *fakeERPClient) ListSupplierPurchaseReceipts(_ context.Context, _, _, _, _ string, _ int) ([]erpnext.PurchaseReceiptDraft, error) {
	return nil, nil
}

func (f *fakeERPClient) CreateDraftPurchaseReceipt(_ context.Context, _, _, _ string, _ erpnext.CreatePurchaseReceiptInput) (erpnext.PurchaseReceiptDraft, error) {
	return erpnext.PurchaseReceiptDraft{}, nil
}

func (f *fakeERPClient) ConfirmAndSubmitPurchaseReceipt(_ context.Context, _, _, _, _ string, _ float64) (erpnext.PurchaseReceiptSubmissionResult, error) {
	return erpnext.PurchaseReceiptSubmissionResult{}, nil
}

func (f *fakeERPClient) UploadSupplierImage(_ context.Context, _, _, _, supplierID, _, _ string, _ []byte) (string, error) {
	fileURL := "/files/" + supplierID + "-avatar.png"
	f.uploadedAvatarURL = fileURL
	for index, item := range f.suppliers {
		if item.ID == supplierID {
			item.Image = fileURL
			f.suppliers[index] = item
			break
		}
	}
	return fileURL, nil
}

func TestServerLoginAndMeFlow(t *testing.T) {
	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   "SUP-001",
		Name:  "Abdulloh",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("failed to generate access credentials: %v", err)
	}

	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{
			suppliers: []erpnext.Supplier{
				{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567"},
			},
		},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
	))
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body, _ := json.Marshal(LoginRequest{
		Phone: "+998901234567",
		Code:  creds.Code,
	})
	resp, err := http.Post(ts.URL+"/v1/mobile/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status: %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("expected token")
	}
	if loginResp.Profile.Role != RoleSupplier || loginResp.Profile.DisplayName != "Abdulloh" {
		t.Fatalf("unexpected profile: %+v", loginResp.Profile)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("me request failed: %v", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected me status: %d", meResp.StatusCode)
	}
}

func TestServerLogoutInvalidatesSession(t *testing.T) {
	server := NewServer(NewERPAuthenticator(
		&fakeERPClient{},
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		nil,
	))
	token, err := server.sessions.Create(Principal{Role: RoleSupplier, DisplayName: "Abdulloh"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/mobile/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("unexpected logout status: %d", logoutResp.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/mobile/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meResp := httptest.NewRecorder()
	server.Handler().ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", meResp.Code)
	}
}

func TestServerProfileUpdateAndAvatarFlow(t *testing.T) {
	fakeERP := &fakeERPClient{
		suppliers: []erpnext.Supplier{
			{ID: "SUP-001", Name: "Abdulloh", Phone: "+998901234567", Image: "/files/original.png"},
		},
	}
	profiles := NewProfileStore(t.TempDir() + "/profile_prefs.json")
	server := NewServer(NewERPAuthenticator(
		fakeERP,
		"http://localhost:8000",
		"key",
		"secret",
		"Stores - CH",
		"10",
		"20",
		"20WERKA0001",
		"+998901111111",
		"Werka",
		profiles,
	))
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	creds, err := suplier.GenerateAccessCredentials(suplier.Supplier{
		Ref:   "SUP-001",
		Name:  "Abdulloh",
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("failed to generate credentials: %v", err)
	}

	loginBody, _ := json.Marshal(LoginRequest{
		Phone: "+998901234567",
		Code:  creds.Code,
	})
	loginResp, err := http.Post(ts.URL+"/v1/mobile/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginPayload LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginPayload); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	updateReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/v1/mobile/profile", bytes.NewReader([]byte(`{"nickname":"Alias"}`)))
	updateReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("profile update request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected profile update status: %d", updateResp.StatusCode)
	}

	var updated Principal
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode profile update response: %v", err)
	}
	if updated.DisplayName != "Alias" {
		t.Fatalf("unexpected nickname after update: %+v", updated)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := part.Write([]byte("pngdata")); err != nil {
		t.Fatalf("failed to write avatar payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	avatarReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/mobile/profile/avatar", &body)
	avatarReq.Header.Set("Authorization", "Bearer "+loginPayload.Token)
	avatarReq.Header.Set("Content-Type", writer.FormDataContentType())
	avatarResp, err := http.DefaultClient.Do(avatarReq)
	if err != nil {
		t.Fatalf("avatar request failed: %v", err)
	}
	defer avatarResp.Body.Close()
	if avatarResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected avatar status: %d", avatarResp.StatusCode)
	}

	var avatarPrincipal Principal
	if err := json.NewDecoder(avatarResp.Body).Decode(&avatarPrincipal); err != nil {
		t.Fatalf("failed to decode avatar response: %v", err)
	}
	if avatarPrincipal.AvatarURL != "http://localhost:8000/files/SUP-001-avatar.png" {
		t.Fatalf("unexpected avatar url: %+v", avatarPrincipal)
	}
	if fakeERP.uploadedAvatarURL == "" {
		t.Fatal("expected fake ERP upload to run")
	}
}

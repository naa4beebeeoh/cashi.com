package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cashi/cashi/backend/internal/domain"
)

type stubService struct {
	payFn    func(ctx context.Context, userID string, amountIDR int64, key string) (domain.PaymentResult, error)
	redeemFn func(ctx context.Context, userID string, amountIDR int64, key string) (domain.RedeemResult, error)
	sumFn    func(ctx context.Context, userID string) (domain.CashbackSummary, error)
}

func (s *stubService) GetCampaign(context.Context) (domain.Campaign, error) {
	return domain.Campaign{ID: domain.CampaignIDFlash, Status: "active"}, nil
}
func (s *stubService) GetSummary(ctx context.Context, userID string) (domain.CashbackSummary, error) {
	if s.sumFn != nil {
		return s.sumFn(ctx, userID)
	}
	return domain.CashbackSummary{UserID: userID, AvailableIDR: 1000}, nil
}
func (s *stubService) ListLedger(context.Context, string) ([]domain.LedgerEntry, error) {
	return nil, nil
}
func (s *stubService) Pay(ctx context.Context, userID string, amountIDR int64, key string) (domain.PaymentResult, error) {
	if s.payFn != nil {
		return s.payFn(ctx, userID, amountIDR, key)
	}
	return domain.PaymentResult{PaymentID: "p1", UserID: userID, AmountIDR: amountIDR, CashbackIDR: 5000}, nil
}
func (s *stubService) Redeem(ctx context.Context, userID string, amountIDR int64, key string) (domain.RedeemResult, error) {
	if s.redeemFn != nil {
		return s.redeemFn(ctx, userID, amountIDR, key)
	}
	return domain.RedeemResult{RedemptionID: "r1", UserID: userID, AmountIDR: amountIDR}, nil
}

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

func TestHealthz(t *testing.T) {
	h := New(&stubService{}, stubPinger{}, stubPinger{}).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestReadyzDependencies(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		h := New(&stubService{}, stubPinger{}, stubPinger{}).Handler()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("postgres down", func(t *testing.T) {
		h := New(&stubService{}, stubPinger{err: errors.New("down")}, stubPinger{}).Handler()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"postgres"`)) {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
	t.Run("redis down", func(t *testing.T) {
		h := New(&stubService{}, stubPinger{}, stubPinger{err: errors.New("down")}).Handler()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"redis"`)) {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
}

func TestCORSPreflight(t *testing.T) {
	h := New(&stubService{}, stubPinger{}, stubPinger{}).Handler()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/payments", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS origin=%q", got)
	}
}

func TestPayHTTPContract(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		body       string
		payErr     error
		replay     bool
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid json",
			headers:    map[string]string{"X-User-ID": "user_a", "Idempotency-Key": "k1"},
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid JSON body",
		},
		{
			name:       "unknown fields rejected",
			headers:    map[string]string{"X-User-ID": "user_a", "Idempotency-Key": "k1"},
			body:       `{"amountIdr":100000,"extra":true}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid JSON body",
		},
		{
			name:       "missing user maps 400",
			headers:    map[string]string{"Idempotency-Key": "k1"},
			body:       `{"amountIdr":100000}`,
			payErr:     domain.ErrMissingUser,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown user maps 404",
			headers:    map[string]string{"X-User-ID": "nope", "Idempotency-Key": "k1"},
			body:       `{"amountIdr":100000}`,
			payErr:     domain.ErrUnknownUser,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "created on fresh pay",
			headers:    map[string]string{"X-User-ID": "user_a", "Idempotency-Key": "k1"},
			body:       `{"amountIdr":100000}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "ok on idempotent replay",
			headers:    map[string]string{"X-User-ID": "user_a", "Idempotency-Key": "k1"},
			body:       `{"amountIdr":100000}`,
			replay:     true,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{
				payFn: func(ctx context.Context, userID string, amountIDR int64, key string) (domain.PaymentResult, error) {
					if tt.payErr != nil {
						return domain.PaymentResult{}, tt.payErr
					}
					return domain.PaymentResult{
						PaymentID:        "p1",
						UserID:           userID,
						AmountIDR:        amountIDR,
						CashbackIDR:      5000,
						IdempotentReplay: tt.replay,
					}, nil
				},
			}
			h := New(svc, stubPinger{}, stubPinger{}).Handler()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if tt.wantErr != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantErr)) {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestRedeemInsufficientFundsConflict(t *testing.T) {
	svc := &stubService{
		redeemFn: func(context.Context, string, int64, string) (domain.RedeemResult, error) {
			return domain.RedeemResult{}, domain.ErrInsufficientFunds
		},
	}
	h := New(svc, stubPinger{}, stubPinger{}).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/redeem", bytes.NewBufferString(`{"amountIdr":999}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_a")
	req.Header.Set("Idempotency-Key", "r1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMeCashbackRequiresMappedErrors(t *testing.T) {
	svc := &stubService{
		sumFn: func(context.Context, string) (domain.CashbackSummary, error) {
			return domain.CashbackSummary{}, domain.ErrMissingUser
		},
	}
	h := New(svc, stubPinger{}, stubPinger{}).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cashback", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWriteErrInternalFallback(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "internal error" {
		t.Fatalf("body=%v", body)
	}
}

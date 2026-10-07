package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/cashi/cashi/backend/internal/cashback"
	"github.com/cashi/cashi/backend/internal/domain"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
)

type Server struct {
	svc   *cashback.Service
	db    *postgres.Store
	redis *redisstore.Store
}

func New(svc *cashback.Service, db *postgres.Store, redis *redisstore.Store) *Server {
	return &Server{svc: svc, db: db, redis: redis}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleInfo)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)
	mux.HandleFunc("GET /api/v1/campaign", s.handleCampaign)
	mux.HandleFunc("GET /api/v1/me/cashback", s.handleMeCashback)
	mux.HandleFunc("GET /api/v1/me/ledger", s.handleMeLedger)
	mux.HandleFunc("POST /api/v1/payments", s.handlePay)
	mux.HandleFunc("POST /api/v1/redeem", s.handleRedeem)
	mux.HandleFunc("OPTIONS /api/v1/", s.handleOptions)
	mux.HandleFunc("OPTIONS /api/v1/{path...}", s.handleOptions)
	return withCORS(mux)
}

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "cashi-api",
		"feature": "flash-cashback",
		"endpoints": []string{
			"/healthz",
			"/readyz",
			"/api/v1/campaign",
			"/api/v1/me/cashback",
			"/api/v1/me/ledger",
			"/api/v1/payments",
			"/api/v1/redeem",
		},
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": "postgres"})
		return
	}
	if err := s.redis.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": "redis"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleCampaign(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.GetCampaign(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleMeCashback(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	summary, err := s.svc.GetSummary(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleMeLedger(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	entries, err := s.svc.ListLedger(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

type amountBody struct {
	AmountIDR int64 `json:"amountIdr"`
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	key := r.Header.Get("Idempotency-Key")
	body, err := decodeAmount(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.svc.Pay(r.Context(), userID, body.AmountIDR, key)
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) handleRedeem(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	key := r.Header.Get("Idempotency-Key")
	body, err := decodeAmount(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.svc.Redeem(r.Context(), userID, body.AmountIDR, key)
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func decodeAmount(r *http.Request) (amountBody, error) {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, 1<<20)
	var body amountBody
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return amountBody{}, errors.New("invalid JSON body")
	}
	return body, nil
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrMissingUser),
		errors.Is(err, domain.ErrMissingIdempotency),
		errors.Is(err, domain.ErrInvalidAmount):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrUnknownUser),
		errors.Is(err, domain.ErrCampaignMissing):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		log.Printf("handler error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	healthHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestCardSummaryHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/cards/primary/summary", nil)
	request.SetPathValue("cardID", "primary")
	response := httptest.NewRecorder()

	cardSummaryHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body cardSummary
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AvailableCredit != 3751.40 {
		t.Fatalf("available credit = %.2f, want 3751.40", body.AvailableCredit)
	}
}

func TestTransactionsHandlerRejectsUnknownCard(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/cards/unknown/transactions", nil)
	request.SetPathValue("cardID", "unknown")
	response := httptest.NewRecorder()

	transactionsHandler(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestPortfolioHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	response := httptest.NewRecorder()

	portfolioHandler(response, request)

	var body portfolio
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Positions) != 3 {
		t.Fatalf("positions = %d, want 3", len(body.Positions))
	}
}

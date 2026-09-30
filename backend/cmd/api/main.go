package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type cardSummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	LastFour        string  `json:"lastFour"`
	Status          string  `json:"status"`
	CurrentBalance  float64 `json:"currentBalance"`
	AvailableCredit float64 `json:"availableCredit"`
	CreditLimit     float64 `json:"creditLimit"`
	PaymentDueDate  string  `json:"paymentDueDate"`
}

type transaction struct {
	ID       string  `json:"id"`
	Merchant string  `json:"merchant"`
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Date     string  `json:"date"`
}

type position struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	Shares      float64 `json:"shares"`
	MarketValue float64 `json:"marketValue"`
	DailyChange float64 `json:"dailyChange"`
}

type portfolio struct {
	Name               string     `json:"name"`
	MarketValue        float64    `json:"marketValue"`
	DailyChange        float64    `json:"dailyChange"`
	DailyChangePercent float64    `json:"dailyChangePercent"`
	Positions          []position `json:"positions"`
}

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", apiInfoHandler)
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /api/v1/cards", cardsHandler)
	mux.HandleFunc("GET /api/v1/cards/{cardID}/summary", cardSummaryHandler)
	mux.HandleFunc("GET /api/v1/cards/{cardID}/transactions", transactionsHandler)
	mux.HandleFunc("GET /api/v1/portfolio", portfolioHandler)
	mux.HandleFunc("GET /api/v1/portfolio/positions", positionsHandler)

	log.Printf("API listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func apiInfoHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "cashi-api",
		"endpoints": []string{
			"/healthz",
			"/api/v1/cards",
			"/api/v1/portfolio",
		},
	})
}

func cardsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []cardSummary{demoCard()})
}

func cardSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("cardID") != demoCard().ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "card not found"})
		return
	}
	writeJSON(w, http.StatusOK, demoCard())
}

func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("cardID") != demoCard().ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "card not found"})
		return
	}
	writeJSON(w, http.StatusOK, demoTransactions())
}

func portfolioHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demoPortfolio())
}

func positionsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, demoPortfolio().Positions)
}

func demoCard() cardSummary {
	return cardSummary{
		ID: "primary", Name: "Cashi Everyday", LastFour: "4821", Status: "active",
		CurrentBalance: 1248.60, AvailableCredit: 3751.40, CreditLimit: 5000,
		PaymentDueDate: "2026-10-18",
	}
}

func demoTransactions() []transaction {
	return []transaction{
		{ID: "txn-001", Merchant: "Whole Foods Market", Category: "Groceries", Amount: 86.42, Date: "2026-09-29"},
		{ID: "txn-002", Merchant: "Metro Coffee", Category: "Food & drink", Amount: 5.80, Date: "2026-09-28"},
		{ID: "txn-003", Merchant: "Northstar Air", Category: "Travel", Amount: 214.00, Date: "2026-09-26"},
	}
}

func demoPortfolio() portfolio {
	return portfolio{
		Name: "Long-term portfolio", MarketValue: 24860.32, DailyChange: 184.76, DailyChangePercent: 0.75,
		Positions: []position{
			{Symbol: "VTI", Name: "Total Stock Market ETF", Shares: 42, MarketValue: 11382.00, DailyChange: 92.40},
			{Symbol: "MSFT", Name: "Microsoft", Shares: 18, MarketValue: 7641.18, DailyChange: 63.72},
			{Symbol: "BND", Name: "Total Bond Market ETF", Shares: 67, MarketValue: 5837.14, DailyChange: 28.64},
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

package domain

import (
	"errors"
	"time"
)

const (
	CampaignIDFlash = "flash_v1"
	RateBPS         = 500 // 5%
	MinPaymentIDR   = int64(20_000)
	DailyCapIDR     = int64(50_000)
	BudgetTotalIDR  = int64(10_000_000)
	CampaignTZ      = "Asia/Jakarta"
)

var (
	ErrInvalidAmount      = errors.New("amount must be a positive integer IDR")
	ErrMissingUser        = errors.New("X-User-ID header is required")
	ErrUnknownUser        = errors.New("unknown user")
	ErrMissingIdempotency = errors.New("Idempotency-Key header is required")
	ErrInsufficientFunds  = errors.New("insufficient cashback balance")
	ErrCampaignMissing    = errors.New("campaign not found")
)

type AwardReason string

const (
	ReasonAwarded      AwardReason = "awarded"
	ReasonBelowMinimum AwardReason = "below_minimum"
	ReasonDailyCap     AwardReason = "daily_cap_reached"
	ReasonBudgetGone   AwardReason = "campaign_budget_exhausted"
	ReasonPartialDaily AwardReason = "partial_daily_cap"
	ReasonPartialBudget AwardReason = "partial_budget"
)

type Campaign struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RateBPS        int    `json:"rateBps"`
	MinPaymentIDR  int64  `json:"minPaymentIdr"`
	DailyCapIDR    int64  `json:"dailyCapIdr"`
	BudgetTotalIDR int64  `json:"budgetTotalIdr"`
	BudgetSpentIDR int64  `json:"budgetSpentIdr"`
	BudgetLeftIDR  int64  `json:"budgetLeftIdr"`
	Status         string `json:"status"`
	Timezone       string `json:"timezone"`
}

type CashbackSummary struct {
	UserID         string `json:"userId"`
	AvailableIDR   int64  `json:"availableIdr"`
	RedeemedIDR    int64  `json:"redeemedIdr"`
	EarnedTodayIDR int64  `json:"earnedTodayIdr"`
	DailyCapIDR    int64  `json:"dailyCapIdr"`
	DailyLeftIDR   int64  `json:"dailyLeftIdr"`
}

type PaymentResult struct {
	PaymentID     string      `json:"paymentId"`
	UserID        string      `json:"userId"`
	AmountIDR     int64       `json:"amountIdr"`
	CashbackIDR   int64       `json:"cashbackIdr"`
	AwardReason   AwardReason `json:"awardReason"`
	AvailableIDR  int64       `json:"availableIdr"`
	EarnedTodayIDR int64      `json:"earnedTodayIdr"`
	BudgetLeftIDR int64       `json:"budgetLeftIdr"`
	IdempotentReplay bool     `json:"idempotentReplay"`
}

type RedeemResult struct {
	RedemptionID     string `json:"redemptionId"`
	UserID           string `json:"userId"`
	AmountIDR        int64  `json:"amountIdr"`
	AvailableIDR     int64  `json:"availableIdr"`
	RedeemedIDR      int64  `json:"redeemedIdr"`
	IdempotentReplay bool   `json:"idempotentReplay"`
}

type LedgerEntry struct {
	ID           string    `json:"id"`
	EntryType    string    `json:"entryType"`
	AmountIDR    int64     `json:"amountIdr"`
	PaymentID    *string   `json:"paymentId,omitempty"`
	RedemptionID *string   `json:"redemptionId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// CashbackForPayment returns the uncapped 5% cashback using integer floor math.
func CashbackForPayment(amountIDR int64) int64 {
	if amountIDR < MinPaymentIDR {
		return 0
	}
	return amountIDR * RateBPS / 10_000
}

func Clamp(n, min, max int64) int64 {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

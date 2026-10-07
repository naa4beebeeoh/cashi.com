package domain

import "testing"

func TestCashbackForPayment(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   int64
	}{
		{"zero", 0, 0},
		{"negative treated below min", -1, 0},
		{"one rupiah", 1, 0},
		{"below minimum", 19_999, 0},
		{"exact minimum", 20_000, 1_000},
		{"floors fractional", 20_010, 1_000},
		{"last amount before cashback ticks to 1001", 20_019, 1_000},
		{"cashback ticks to 1001", 20_020, 1_001},
		{"typical payment", 100_000, 5_000},
		{"large payment", 1_000_000, 50_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CashbackForPayment(tt.amount); got != tt.want {
				t.Fatalf("CashbackForPayment(%d)=%d want %d", tt.amount, got, tt.want)
			}
		})
	}
}

func TestComputeAward(t *testing.T) {
	// Every case states AmountIDR / EarnedTodayIDR / BudgetSpentIDR explicitly (no hidden zeros).
	tests := []struct {
		name           string
		amountIDR      int64
		earnedTodayIDR int64
		budgetSpentIDR int64
		status         CampaignStatus // empty → active
		want           AwardDecision
	}{
		{
			name:           "below minimum",
			amountIDR:      19_999,
			earnedTodayIDR: 0,
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonBelowMinimum},
		},
		{
			name:           "full award",
			amountIDR:      100_000, // raw 5000
			earnedTodayIDR: 0,
			budgetSpentIDR: 0,
			want:           AwardDecision{5_000, ReasonAwarded},
		},
		{
			name:           "daily cap already reached",
			amountIDR:      100_000,
			earnedTodayIDR: DailyCapIDR,
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonDailyCap},
		},
		{
			name:           "partial daily cap",
			amountIDR:      100_000,              // raw 5000
			earnedTodayIDR: DailyCapIDR - 2_000, // daily left 2000
			budgetSpentIDR: 0,
			want:           AwardDecision{2_000, ReasonPartialDaily},
		},
		{
			name:           "budget exhausted by spent",
			amountIDR:      100_000,
			earnedTodayIDR: 0,
			budgetSpentIDR: BudgetTotalIDR,
			want:           AwardDecision{0, ReasonBudgetGone},
		},
		{
			name:           "campaign status exhausted",
			amountIDR:      100_000,
			earnedTodayIDR: 0,
			budgetSpentIDR: 0,
			status:         CampaignStatusExhausted,
			want:           AwardDecision{0, ReasonBudgetGone},
		},
		{
			name:           "budget clamp wins when tighter than daily",
			amountIDR:      200_000,                 // raw 10000
			earnedTodayIDR: DailyCapIDR - 5_000,    // daily left 5000
			budgetSpentIDR: BudgetTotalIDR - 3_000, // budget left 3000
			want:           AwardDecision{3_000, ReasonPartialBudget},
		},
		{
			name:           "daily clamp when tighter than budget",
			amountIDR:      200_000,              // raw 10000
			earnedTodayIDR: DailyCapIDR - 3_000, // daily left 3000
			budgetSpentIDR: 0,                   // budget plentiful
			want:           AwardDecision{3_000, ReasonPartialDaily},
		},
		{
			name:           "over-earned daily row clamps left to zero",
			amountIDR:      100_000,
			earnedTodayIDR: DailyCapIDR + 5_000, // corrupt / over-cap row
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonDailyCap},
		},
		{
			name:           "below minimum even when campaign exhausted",
			amountIDR:      19_999,
			earnedTodayIDR: 0,
			budgetSpentIDR: BudgetTotalIDR,
			status:         CampaignStatusExhausted,
			want:           AwardDecision{0, ReasonBelowMinimum},
		},
		{
			name:           "below minimum even when daily already full",
			amountIDR:      19_999,
			earnedTodayIDR: DailyCapIDR,
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonBelowMinimum},
		},
		{
			name:           "whale payment cannot exceed remaining campaign budget",
			amountIDR:      50_000_000, // raw 2_500_000
			earnedTodayIDR: 0,
			budgetSpentIDR: BudgetTotalIDR - 7_000, // budget left 7000
			want:           AwardDecision{7_000, ReasonPartialBudget},
		},
		{
			name:           "whale payment cannot exceed remaining daily cap",
			amountIDR:      50_000_000,           // raw 2_500_000
			earnedTodayIDR: DailyCapIDR - 4_000, // daily left 4000
			budgetSpentIDR: 0,
			want:           AwardDecision{4_000, ReasonPartialDaily},
		},
		{
			name:           "exact daily remainder is awarded not partial",
			amountIDR:      100_000, // raw 5000
			earnedTodayIDR: DailyCapIDR - 5_000,
			budgetSpentIDR: 0,
			want:           AwardDecision{5_000, ReasonAwarded},
		},
		{
			name:           "exact budget remainder is awarded not partial",
			amountIDR:      100_000, // raw 5000
			earnedTodayIDR: 0,
			budgetSpentIDR: BudgetTotalIDR - 5_000,
			want:           AwardDecision{5_000, ReasonAwarded},
		},
		{
			name:           "equal daily and budget leftover prefers partial_budget reason",
			amountIDR:      200_000, // raw 10000
			earnedTodayIDR: DailyCapIDR - 3_000,
			budgetSpentIDR: BudgetTotalIDR - 3_000,
			want:           AwardDecision{3_000, ReasonPartialBudget},
		},
		{
			name:           "budget spent past total yields zero",
			amountIDR:      100_000,
			earnedTodayIDR: 0,
			budgetSpentIDR: BudgetTotalIDR + 1,
			want:           AwardDecision{0, ReasonBudgetGone},
		},
		{
			name:           "zero amount",
			amountIDR:      0,
			earnedTodayIDR: 0,
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonBelowMinimum},
		},
		{
			name:           "negative amount",
			amountIDR:      -100_000,
			earnedTodayIDR: 0,
			budgetSpentIDR: 0,
			want:           AwardDecision{0, ReasonBelowMinimum},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := CampaignStatusActive
			if tt.status != "" {
				status = tt.status
			}
			got := ComputeAward(AwardInput{
				AmountIDR:      tt.amountIDR,
				EarnedTodayIDR: tt.earnedTodayIDR,
				BudgetSpentIDR: tt.budgetSpentIDR,
				MinPaymentIDR:  MinPaymentIDR,
				RateBPS:        RateBPS,
				DailyCapIDR:    DailyCapIDR,
				BudgetTotalIDR: BudgetTotalIDR,
				CampaignStatus: status,
			})
			if got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestClampAndMinInt64(t *testing.T) {
	if got := Clamp(5, 0, 10); got != 5 {
		t.Fatalf("Clamp mid=%d", got)
	}
	if got := Clamp(-1, 0, 10); got != 0 {
		t.Fatalf("Clamp low=%d", got)
	}
	if got := Clamp(99, 0, 10); got != 10 {
		t.Fatalf("Clamp high=%d", got)
	}
	if got := MinInt64(3, 7); got != 3 {
		t.Fatalf("MinInt64=%d", got)
	}
	if got := MinInt64(9, 2); got != 2 {
		t.Fatalf("MinInt64=%d", got)
	}
}

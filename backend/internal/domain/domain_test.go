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
	base := AwardInput{
		MinPaymentIDR:  MinPaymentIDR,
		RateBPS:        RateBPS,
		DailyCapIDR:    DailyCapIDR,
		BudgetTotalIDR: BudgetTotalIDR,
		CampaignStatus: "active",
	}

	tests := []struct {
		name string
		mod  func(*AwardInput)
		want AwardDecision
	}{
		{
			name: "below minimum",
			mod:  func(in *AwardInput) { in.AmountIDR = 19_999 },
			want: AwardDecision{0, ReasonBelowMinimum},
		},
		{
			name: "full award",
			mod:  func(in *AwardInput) { in.AmountIDR = 100_000 },
			want: AwardDecision{5_000, ReasonAwarded},
		},
		{
			name: "daily cap already reached",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000
				in.EarnedTodayIDR = DailyCapIDR
			},
			want: AwardDecision{0, ReasonDailyCap},
		},
		{
			name: "partial daily cap",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000 // raw 5000
				in.EarnedTodayIDR = 48_000 // left 2000
			},
			want: AwardDecision{2_000, ReasonPartialDaily},
		},
		{
			name: "budget exhausted by spent",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000
				in.BudgetSpentIDR = BudgetTotalIDR
			},
			want: AwardDecision{0, ReasonBudgetGone},
		},
		{
			name: "campaign status exhausted",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000
				in.CampaignStatus = "exhausted"
			},
			want: AwardDecision{0, ReasonBudgetGone},
		},
		{
			name: "partial budget",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000 // raw 5000
				in.BudgetSpentIDR = BudgetTotalIDR - 1_500
			},
			want: AwardDecision{1_500, ReasonPartialBudget},
		},
		{
			name: "budget clamp wins when tighter than daily",
			mod: func(in *AwardInput) {
				in.AmountIDR = 200_000 // raw 10000
				in.EarnedTodayIDR = 45_000 // daily left 5000
				in.BudgetSpentIDR = BudgetTotalIDR - 3_000
			},
			want: AwardDecision{3_000, ReasonPartialBudget},
		},
		{
			name: "daily clamp when tighter than budget",
			mod: func(in *AwardInput) {
				in.AmountIDR = 200_000 // raw 10000
				in.EarnedTodayIDR = 47_000 // daily left 3000
				in.BudgetSpentIDR = 0
			},
			want: AwardDecision{3_000, ReasonPartialDaily},
		},
		{
			name: "over-earned daily row clamps left to zero",
			mod: func(in *AwardInput) {
				in.AmountIDR = 100_000
				in.EarnedTodayIDR = DailyCapIDR + 5_000
			},
			want: AwardDecision{0, ReasonDailyCap},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.mod(&in)
			got := ComputeAward(in)
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

package domain

import "testing"

func TestCashbackForPayment(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   int64
	}{
		{"below minimum", 19_999, 0},
		{"exact minimum", 20_000, 1_000},
		{"floors fractional", 20_010, 1_000},
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

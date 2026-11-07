package finance

import "testing"

func TestCalculateMetricsUsesFinancialData(t *testing.T) {
	sections := &FinancialSections{
		BalanceSheet: map[string]string{
			"Assets":      "1000",
			"Liabilities": "300",
			"Equity":      "700",
		},
		IncomeStatement: map[string]string{
			"Revenue":         "500",
			"GrossProfit":     "250",
			"OperatingProfit": "150",
			"ProfitLoss":      "100",
		},
	}

	metrics := CalculateMetrics(sections, 20, 100)
	if metrics.GPM == 0 {
		t.Fatalf("expected positive GPM, got %v", metrics.GPM)
	}
	if metrics.PER == 0 {
		t.Fatalf("expected positive PER, got %v", metrics.PER)
	}
}

func TestCalculateMetricsDerivesSharesOutstandingFromEquityAndPrice(t *testing.T) {
	sections := &FinancialSections{
		BalanceSheet: map[string]string{
			"Assets":      "1000",
			"Liabilities": "300",
			"Equity":      "700",
		},
		IncomeStatement: map[string]string{
			"Revenue":         "500",
			"GrossProfit":     "250",
			"OperatingProfit": "150",
			"ProfitLoss":      "100",
		},
	}

	metrics := CalculateMetrics(sections, 20000, 0)
	if metrics.PBV <= 0 {
		t.Fatalf("expected derived PBV to be positive without manual shares, got %v", metrics.PBV)
	}
	if metrics.MOS <= 0 {
		t.Fatalf("expected positive margin of safety, got %v", metrics.MOS)
	}
}

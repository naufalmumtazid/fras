package finance

type FinancialSections struct {
	BalanceSheet    map[string]string `json:"balance_sheet"`
	IncomeStatement map[string]string `json:"income_statement"`
	CashFlow        map[string]string `json:"cash_flow"`
}

type FinancialMetrics struct {
	GPM  float64 `json:"gpm_percent"`
	NPM  float64 `json:"npm_percent"`
	OPM  float64 `json:"opm_percent"`
	ROA  float64 `json:"roa_percent"`
	ROE  float64 `json:"roe_percent"`
	DER  float64 `json:"der"`
	TATO float64 `json:"tato"`
	PER  float64 `json:"per"`
	PBV  float64 `json:"pbv"`
}

type StatSummary struct {
	Median float64 `json:"median"`
	Mean   float64 `json:"mean"`
}

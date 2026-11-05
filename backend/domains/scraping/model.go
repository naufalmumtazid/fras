package scraping

import "backend/domains/finance"

type ReportTarget struct {
	URL     string
	Quarter string
	Year    int
}

type AnalyticResult struct {
	URL     string                     `json:"url"`
	Quarter string                     `json:"quarter"`
	Year    int                        `json:"year"`
	Status  string                     `json:"status"`
	Data    *finance.FinancialSections `json:"data,omitempty"`
	Metrics *finance.FinancialMetrics  `json:"metrics,omitempty"`
}

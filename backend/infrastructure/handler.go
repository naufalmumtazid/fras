package infrastructure

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/domains/analytics"
)

type EmitenRequest struct {
	EmitenCode        string   `json:"emiten_code"`
	StartYear         *int     `json:"start_year"`
	StartQuarter      *int     `json:"start_quarter"`
	QuarterCount      *int     `json:"quarter_count"`
	SharePrice        *float64 `json:"share_price,omitempty"`
	SharesOutstanding *float64 `json:"shares_outstanding,omitempty"`
}

func getDefaultStartPeriod() (int, int) {
	now := time.Now()
	year := now.Year()
	month := now.Month()

	var quarter int
	switch {
	case month >= time.January && month <= time.March:
		quarter = 3
		year--
	case month >= time.April && month <= time.June:
		quarter = 4
		year--
	case month >= time.July && month <= time.September:
		quarter = 1
	case month >= time.October && month <= time.November:
		quarter = 3
	default:
		quarter = 3
	}
	return year, quarter
}

func HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmitenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EmitenCode == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	defaultYear, defaultQuarter := getDefaultStartPeriod()
	startYear := defaultYear
	if req.StartYear != nil {
		startYear = *req.StartYear
	}

	startQuarter := defaultQuarter
	if req.StartQuarter != nil {
		startQuarter = *req.StartQuarter
	}

	quarterCount := 5
	if req.QuarterCount != nil {
		quarterCount = *req.QuarterCount
	}

	sharePrice := 0.0
	if req.SharePrice != nil {
		sharePrice = *req.SharePrice
	}

	sharesOutstanding := 0.0
	if req.SharesOutstanding != nil {
		sharesOutstanding = *req.SharesOutstanding
	}

	// Panggil Service Analytics yang baru dibuat
	analyticsService := analytics.NewAnalyticsService()
	globalSummary, quarterlyData, err := analyticsService.ProcessEmitenAnalytics(
		req.EmitenCode, startYear, startQuarter, quarterCount, sharePrice, sharesOutstanding,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"emiten":         req.EmitenCode,
		"global_summary": globalSummary,
		"quarterly_data": quarterlyData,
	})
}

package handler

import (
	"encoding/json"
	"net/http"

	"fras/domains/analytics"
	"fras/infrastructure"
)

type EmitenRequest struct {
	EmitenCode        string   `json:"emiten_code"`
	StartYear         *int     `json:"start_year"`
	StartQuarter      *int     `json:"start_quarter"`
	QuarterCount      *int     `json:"quarter_count"`
	SharePrice        *float64 `json:"share_price,omitempty"`
	SharesOutstanding *float64 `json:"shares_outstanding,omitempty"`
}

// HandleAnalytics processes stock analysis for a given IDX emiten.
// @Summary Analyze IDX Emiten Financial Report
// @Description Fetches iXBRL financial report data from IDX, calculates financial ratios (NPM, ROA, ROE, DER, PER, PBV), and computes global statistics (Median & Mean).
// @Tags Analytics
// @Accept json
// @Produce json
// @Param request body EmitenRequest true "Payload containing emiten code and current share price"
// @Success 200 {object} map[string]interface{} "Successfully processed financial analysis"
// @Failure 400 {string} string "Invalid request body"
// @Failure 500 {string} string "Internal server error"
// @Router /api/v1/analyze-stock [post]
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

	defaultYear, defaultQuarter := infrastructure.GetDefaultStartPeriod()
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

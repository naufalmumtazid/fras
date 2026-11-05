package analytics

import "backend/domains/finance"

type SummaryMetric = finance.StatSummary

type GlobalSummary struct {
	GPM  SummaryMetric `json:"gpm_percent"`
	NPM  SummaryMetric `json:"npm_percent"`
	OPM  SummaryMetric `json:"opm_percent"`
	ROA  SummaryMetric `json:"roa_percent"`
	ROE  SummaryMetric `json:"roe_percent"`
	DER  SummaryMetric `json:"der"`
	TATO SummaryMetric `json:"tato"`
	PER  SummaryMetric `json:"per"`
	PBV  SummaryMetric `json:"pbv"`
}

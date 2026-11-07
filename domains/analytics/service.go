package analytics

import (
	"fras/domains/finance"
	"fras/domains/scraping"
)

type AnalyticsService struct{}

func NewAnalyticsService() *AnalyticsService {
	return &AnalyticsService{}
}

func (s *AnalyticsService) ProcessEmitenAnalytics(code string, startYear int, startQuarter int, quarterCount int, sharePrice float64, sharesOutstanding float64) (GlobalSummary, []scraping.AnalyticResult, error) {
	results, err := scraping.FetchQuarterlyReports(code, startYear, startQuarter, quarterCount, sharePrice, sharesOutstanding)
	if err != nil {
		return GlobalSummary{}, nil, err
	}

	var gpmList, npmList, opmList, roaList, roeList, derList, tatoList, perList, pbvList, mosList []float64
	for _, res := range results {
		if res.Metrics != nil {
			gpmList = append(gpmList, res.Metrics.GPM)
			npmList = append(npmList, res.Metrics.NPM)
			opmList = append(opmList, res.Metrics.OPM)
			roaList = append(roaList, res.Metrics.ROA)
			roeList = append(roeList, res.Metrics.ROE)
			derList = append(derList, res.Metrics.DER)
			tatoList = append(tatoList, res.Metrics.TATO)
			perList = append(perList, res.Metrics.PER)
			pbvList = append(pbvList, res.Metrics.PBV)
			mosList = append(mosList, res.Metrics.MOS)
		}
	}

	return GlobalSummary{
		GPM:  finance.ComputeStat(gpmList),
		NPM:  finance.ComputeStat(npmList),
		OPM:  finance.ComputeStat(opmList),
		ROA:  finance.ComputeStat(roaList),
		ROE:  finance.ComputeStat(roeList),
		DER:  finance.ComputeStat(derList),
		TATO: finance.ComputeStat(tatoList),
		PER:  finance.ComputeStat(perList),
		PBV:  finance.ComputeStat(pbvList),
		MOS:  finance.ComputeStat(mosList),
	}, results, nil
}

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
)

type EmitenRequest struct {
	EmitenCode        string   `json:"emiten_code"`
	StartYear         *int     `json:"start_year"`
	StartQuarter      *int     `json:"start_quarter"`
	QuarterCount      *int     `json:"quarter_count"`
	SharePrice        *float64 `json:"share_price,omitempty"`        // Harga saham saat ini
	SharesOutstanding *float64 `json:"shares_outstanding,omitempty"` // Jumlah lembar saham (misal: 7786 untuk 7,78 miliar lembar)
}

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

type GlobalSummary struct {
	GPM  StatSummary `json:"gpm_percent"`
	NPM  StatSummary `json:"npm_percent"`
	OPM  StatSummary `json:"opm_percent"`
	ROA  StatSummary `json:"roa_percent"`
	ROE  StatSummary `json:"roe_percent"`
	DER  StatSummary `json:"der"`
	TATO StatSummary `json:"tato"`
	PER  StatSummary `json:"per"`
	PBV  StatSummary `json:"pbv"`
}

type AnalyticResult struct {
	URL     string             `json:"url"`
	Quarter string             `json:"quarter"`
	Year    int                `json:"year"`
	Status  string             `json:"status"`
	Data    *FinancialSections `json:"data,omitempty"`
	Metrics *FinancialMetrics  `json:"metrics,omitempty"`
}

type ReportTarget struct {
	URL     string
	Quarter string
	Year    int
}

var (
	cachedCookies []*http.Cookie
	cookieTime    time.Time
	cookieMutex   sync.Mutex
)

func getValidCookies() ([]*http.Cookie, error) {
	cookieMutex.Lock()
	defer cookieMutex.Unlock()

	if len(cachedCookies) > 0 && time.Since(cookieTime) < 20*time.Minute {
		return cachedCookies, nil
	}

	browser := rod.New().NoDefaultDevice().MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("https://www.idx.co.id/")
	page.MustWaitLoad()
	time.Sleep(2 * time.Second)

	rodCookies, err := page.Cookies([]string{"https://www.idx.co.id"})
	if err != nil {
		return nil, err
	}

	var httpCookies []*http.Cookie
	for _, rc := range rodCookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     rc.Name,
			Value:    rc.Value,
			Domain:   rc.Domain,
			Path:     rc.Path,
			Secure:   rc.Secure,
			HttpOnly: rc.HTTPOnly,
		})
	}
	cachedCookies = httpCookies
	cookieTime = time.Now()

	return cachedCookies, nil
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

func generateUrls(code string, startYear int, startQuarter int, quarterCount int) []ReportTarget {
	code = strings.ToUpper(code)
	currentYear := startYear
	currentQuarter := startQuarter

	var targets []ReportTarget
	for i := 0; i < quarterCount; i++ {
		var qStr string
		if currentQuarter == 4 {
			qStr = "Audit"
		} else {
			qStr = fmt.Sprintf("TW%d", currentQuarter)
		}

		var u string
		if qStr == "Audit" {
			u = fmt.Sprintf("https://www.idx.co.id/Portals/0/StaticData/ListedCompanies/Corporate_Actions/New_Info_JSX/Jenis_Informasi/01_Laporan_Keuangan/02_Soft_Copy_Laporan_Keuangan//Laporan%%20Keuangan%%20Tahun%%20%d/Audit/%s/inlineXBRL.zip", currentYear, code)
		} else {
			u = fmt.Sprintf("https://www.idx.co.id/Portals/0/StaticData/ListedCompanies/Corporate_Actions/New_Info_JSX/Jenis_Informasi/01_Laporan_Keuangan/02_Soft_Copy_Laporan_Keuangan//Laporan%%20Keuangan%%20Tahun%%20%d/%s/%s/inlineXBRL.zip", currentYear, qStr, code)
		}

		targets = append(targets, ReportTarget{
			URL:     u,
			Quarter: qStr,
			Year:    currentYear,
		})

		currentQuarter--
		if currentQuarter < 1 {
			currentQuarter = 4
			currentYear--
		}
	}

	return targets
}

func parseVal(s string) float64 {
	clean := strings.ReplaceAll(s, ",", "")
	clean = strings.ReplaceAll(clean, " ", "")
	val, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0.0
	}
	return val
}

func findAccountValue(m map[string]string, candidateKeys []string) float64 {
	for _, key := range candidateKeys {
		if valStr, exists := m[key]; exists {
			if val := parseVal(valStr); val != 0 {
				return val
			}
		}
	}

	for k, v := range m {
		if strings.HasSuffix(strings.ToLower(k), "textblock") {
			continue
		}

		lowerK := strings.ToLower(k)
		for _, kw := range candidateKeys {
			if strings.EqualFold(lowerK, strings.ToLower(kw)) {
				if val := parseVal(v); val != 0 {
					return val
				}
			}
		}
	}
	return 0.0
}

func calculateFinancialMetrics(fs *FinancialSections, sharePrice float64, sharesOutstandingInput float64) FinancialMetrics {
	bs := fs.BalanceSheet
	is := fs.IncomeStatement

	revenue := findAccountValue(is, []string{"SalesAndRevenue", "RevenueFromContractsWithCustomers", "Revenue", "Sales"})
	grossProfit := findAccountValue(is, []string{"GrossProfit"})
	operatingProfit := findAccountValue(is, []string{"OperatingProfit"})
	netProfit := findAccountValue(is, []string{"ProfitLossAttributableToParentEntity", "ProfitLoss"})

	totalAssets := findAccountValue(bs, []string{"Assets"})
	totalLiabilities := findAccountValue(bs, []string{"Liabilities"})
	totalEquity := findAccountValue(bs, []string{"EquityAttributableToEquityOwnersOfParentEntity", "Equity"})

	var gpm, npm, opm, roa, roe, der, tato float64

	if revenue > 0 {
		gpm = (grossProfit / revenue) * 100
		npm = (netProfit / revenue) * 100
		opm = (operatingProfit / revenue) * 100
		tato = revenue / totalAssets
	}

	if totalAssets > 0 {
		roa = (netProfit / totalAssets) * 100
	}

	if totalEquity > 0 {
		roe = (netProfit / totalEquity) * 100
		der = totalLiabilities / totalEquity
	}

	var per, pbv float64
	if sharePrice > 0 && sharesOutstandingInput > 0 {
		// Karena Data & Input sama-sama dalam skala Miliar (Billion),
		// perhitungan EPS & BVPS langsung akurat tanpa pengali besar.
		sharesBillion := sharesOutstandingInput / 1000.0 // Konversi juta lembar input ke miliar lembar

		if sharesBillion > 0 {
			eps := netProfit / sharesBillion
			bvps := totalEquity / sharesBillion

			if eps > 0 {
				per = sharePrice / eps
			}
			if bvps > 0 {
				pbv = sharePrice / bvps
			}
		}
	}

	return FinancialMetrics{
		GPM:  math.Round(gpm*100) / 100,
		NPM:  math.Round(npm*100) / 100,
		OPM:  math.Round(opm*100) / 100,
		ROA:  math.Round(roa*100) / 100,
		ROE:  math.Round(roe*100) / 100,
		DER:  math.Round(der*100) / 100,
		TATO: math.Round(tato*100) / 100,
		PER:  math.Round(per*100) / 100,
		PBV:  math.Round(pbv*100) / 100,
	}
}

func calculateMedian(values []float64) float64 {
	var filtered []float64
	for _, v := range values {
		if v != 0 {
			filtered = append(filtered, v)
		}
	}

	n := len(filtered)
	if n == 0 {
		return 0
	}

	sort.Float64s(filtered)

	if n%2 == 1 {
		return math.Round(filtered[n/2]*100) / 100
	}

	mid := n / 2
	median := (filtered[mid-1] + filtered[mid]) / 2.0
	return math.Round(median*100) / 100
}

func calculateMean(values []float64) float64 {
	sum := 0.0
	count := 0
	for _, v := range values {
		if v != 0 {
			sum += v
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return math.Round((sum/float64(count))*100) / 100
}

func computeStat(values []float64) StatSummary {
	return StatSummary{
		Median: calculateMedian(values),
		Mean:   calculateMean(values),
	}
}

func pullXBRLBEI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmitenRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.EmitenCode == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cookies, err := getValidCookies()
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal inisialisasi sesi browser: %v", err), http.StatusInternalServerError)
		return
	}

	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://www.idx.co.id")
	if len(cookies) > 0 {
		jar.SetCookies(u, cookies)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	defaultYear, defaultQuarter := getDefaultStartPeriod()
	defaultQuarterCount := 5

	startYear := defaultYear
	if req.StartYear != nil {
		startYear = *req.StartYear
	}

	startQuarter := defaultQuarter
	if req.StartQuarter != nil {
		startQuarter = *req.StartQuarter
	}

	quarterCount := defaultQuarterCount
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

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	targets := generateUrls(req.EmitenCode, startYear, startQuarter, quarterCount)

	results := make([]AnalyticResult, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t ReportTarget) {
			defer wg.Done()

			httpReq, _ := http.NewRequest("GET", t.URL, nil)
			httpReq.Header.Set("User-Agent", userAgent)
			httpReq.Header.Set("Referer", "https://www.idx.co.id/")
			httpReq.Header.Set("Accept", "application/zip,application/octet-stream,*/*;q=0.8")

			resp, err := client.Do(httpReq)
			if err != nil || resp.StatusCode != 200 {
				statusCode := 0
				if resp != nil {
					statusCode = resp.StatusCode
				}
				results[idx] = AnalyticResult{
					URL:     t.URL,
					Quarter: t.Quarter,
					Year:    t.Year,
					Status:  fmt.Sprintf("Gagal (HTTP Status: %v)", statusCode),
				}
				if resp != nil {
					resp.Body.Close()
				}
				return
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil || bytes.Contains(bodyBytes[:min(len(bodyBytes), 100)], []byte("<html")) {
				results[idx] = AnalyticResult{
					URL:     t.URL,
					Quarter: t.Quarter,
					Year:    t.Year,
					Status:  "Gagal: Terkena blokir / HTML Error",
				}
				return
			}

			zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
			if err != nil {
				results[idx] = AnalyticResult{
					URL:     t.URL,
					Quarter: t.Quarter,
					Year:    t.Year,
					Status:  "Gagal: File ZIP Corrupted",
				}
				return
			}

			balanceSheet := make(map[string]string)
            incomeStatement := make(map[string]string)
            cashFlow := make(map[string]string)

			for _, f := range zipReader.File {
				lowerName := strings.ToLower(f.Name)
				if strings.HasSuffix(lowerName, ".xml") || strings.HasSuffix(lowerName, ".xbrl") || strings.HasSuffix(lowerName, ".htm") || strings.HasSuffix(lowerName, ".html") {
					rc, err := f.Open()
					if err != nil {
						continue
					}

					doc, err := goquery.NewDocumentFromReader(rc)
					rc.Close()
					if err != nil {
						continue
					}

					doc.Find("[name]").Each(func(i int, s *goquery.Selection) {
						name, exists := s.Attr("name")
						if !exists {
							return
						}

						lowerAttrName := strings.ToLower(name)
						valStr := strings.TrimSpace(s.Text())
						if valStr == "" {
							return
						}

						// Konversi nilai finansial mentah ke Miliar (dibagi 1,000 dari skala jutaan/ribuan IDX)
						valFloat := parseVal(valStr)
						valBillion := valFloat / 1000.0
						formattedBillionStr := fmt.Sprintf("%.2f", valBillion)

						parts := strings.Split(name, ":")
						shortName := parts[len(parts)-1]

						isBalanceSheet := strings.Contains(lowerAttrName, "asset") ||
							strings.Contains(lowerAttrName, "liabilities") ||
							strings.Contains(lowerAttrName, "equity")

						isIncomeStatement := strings.Contains(lowerAttrName, "revenue") ||
							strings.Contains(lowerAttrName, "profit") ||
							strings.Contains(lowerAttrName, "loss") ||
							strings.Contains(lowerAttrName, "income") ||
							strings.Contains(lowerAttrName, "sales")

						isCashFlow := strings.Contains(lowerAttrName, "cash") ||
							strings.Contains(lowerAttrName, "operatingactivities") ||
							strings.Contains(lowerAttrName, "investingactivities") ||
							strings.Contains(lowerAttrName, "financingactivities")

						if isBalanceSheet {
							balanceSheet[shortName] = formattedBillionStr
						}
						if isIncomeStatement {
							incomeStatement[shortName] = formattedBillionStr
						}
						if isCashFlow {
							cashFlow[shortName] = formattedBillionStr
						}
					})
				}
			}

			statusMsg := "Sukses"
			hasData := len(balanceSheet) > 0 || len(incomeStatement) > 0 || len(cashFlow) > 0

			var finData *FinancialSections
			var metrics *FinancialMetrics
			if hasData {
				finData = &FinancialSections{
					BalanceSheet:    balanceSheet,
					IncomeStatement: incomeStatement,
					CashFlow:        cashFlow,
				}

				calcMetrics := calculateFinancialMetrics(finData, sharePrice, sharesOutstanding)
				metrics = &calcMetrics
			} else {
				statusMsg = "Sukses (Namun data section kosong / tidak cocok)"
			}

			results[idx] = AnalyticResult{
				URL:     t.URL,
				Quarter: t.Quarter,
				Year:    t.Year,
				Status:  statusMsg,
				Data:    finData,
				Metrics: metrics,
			}
		}(i, target)
	}

	wg.Wait()

	var gpmList, npmList, opmList, roaList, roeList, derList, tatoList, perList, pbvList []float64

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
		}
	}

	dataGlobalSummary := GlobalSummary{
		GPM:  computeStat(gpmList),
		NPM:  computeStat(npmList),
		OPM:  computeStat(opmList),
		ROA:  computeStat(roaList),
		ROE:  computeStat(roeList),
		DER:  computeStat(derList),
		TATO: computeStat(tatoList),
		PER:  computeStat(perList),
		PBV:  computeStat(pbvList),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"emiten":         req.EmitenCode,
		"global_summary": dataGlobalSummary,
		"quarterly_data": results,
	})
}

func main() {
	http.HandleFunc("/analytics-emiten", pullXBRLBEI)
	fmt.Println("Financial Analytics Server started successfully at port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}
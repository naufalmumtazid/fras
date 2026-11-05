package scraping

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"backend/domains/finance"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
)

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

func FetchQuarterlyReports(code string, startYear int, startQuarter int, quarterCount int, sharePrice float64, sharesOutstanding float64) ([]AnalyticResult, error) {
	cookies, err := getValidCookies()
	if err != nil {
		return nil, err
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

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	targets := generateUrls(code, startYear, startQuarter, quarterCount)
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
			if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
				statusCode := 0
				if resp != nil {
					statusCode = resp.StatusCode
					resp.Body.Close()
				}
				results[idx] = AnalyticResult{
					URL:     t.URL,
					Quarter: t.Quarter,
					Year:    t.Year,
					Status:  fmt.Sprintf("Gagal (HTTP Status: %v)", statusCode),
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

						valFloat := finance.ParseVal(valStr)
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

			var finData *finance.FinancialSections
			var metrics *finance.FinancialMetrics
			if hasData {
				finData = &finance.FinancialSections{
					BalanceSheet:    balanceSheet,
					IncomeStatement: incomeStatement,
					CashFlow:        cashFlow,
				}
				calcMetrics := finance.CalculateMetrics(finData, sharePrice, sharesOutstanding)
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
	return results, nil
}

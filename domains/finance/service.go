package finance

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

type FinanceService struct{}

func NewFinanceService() *FinanceService {
  return &FinanceService{}
}

func (s *FinanceService) CalculateMetrics(fs *FinancialSections, sharePrice float64, sharesOutstandingInput float64) FinancialMetrics {
  return CalculateMetrics(fs, sharePrice, sharesOutstandingInput)
}

func (s *FinanceService) ComputeStat(values []float64) StatSummary {
  return ComputeStat(values)
}

func CalculateMetrics(fs *FinancialSections, sharePrice float64, sharesOutstandingInput float64) FinancialMetrics {
  bs := fs.BalanceSheet
  is := fs.IncomeStatement

  revenue := FindAccountValue(is, []string{"SalesAndRevenue", "RevenueFromContractsWithCustomers", "Revenue", "Sales"})
  grossProfit := FindAccountValue(is, []string{"GrossProfit"})
  operatingProfit := FindAccountValue(is, []string{"OperatingProfit"})
  netProfit := FindAccountValue(is, []string{"ProfitLossAttributableToParentEntity", "ProfitLoss"})

  totalAssets := FindAccountValue(bs, []string{"Assets"})
  totalLiabilities := FindAccountValue(bs, []string{"Liabilities"})
  totalEquity := FindAccountValue(bs, []string{"EquityAttributableToEquityOwnersOfParentEntity", "Equity"})

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

  var per, pbv, mos float64
  sharesOutstanding := sharesOutstandingInput
  if sharesOutstanding <= 0 {
    // Coba ambil jumlah saham beredar dari neraca jika input kosong
    sharesOutstanding = FindAccountValue(bs, []string{"NumberOfSharesIssued", "SharesOutstanding", "OrdinaryShares", "Shares"})
  }

  if sharePrice > 0 && sharesOutstanding > 0 {
    sharesBillion := sharesOutstanding / 1000.0
    if sharesBillion > 0 {
      eps := netProfit / sharesBillion
      bvps := totalEquity / sharesBillion
      
      if eps > 0 {
        per = sharePrice / eps
      }
      if bvps > 0 {
        pbv = sharePrice / bvps
      }
      
      // Menghitung Nilai Intrinsik & Margin of Safety menggunakan Graham Number
      if eps > 0 && bvps > 0 {
        intrinsicValue := deriveIntrinsicValuePerShare(eps, bvps)
        if intrinsicValue > 0 && intrinsicValue > sharePrice {
          mos = ((intrinsicValue - sharePrice) / intrinsicValue) * 100
        }
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
    PER:  roundToPrecision(per, 4),
    PBV:  roundToPrecision(pbv, 4),
    MOS:  roundToPrecision(mos, 4),
  }
}

func roundToPrecision(value float64, precision int) float64 {
  if value == 0 {
    return 0
  }
  factor := math.Pow10(precision)
  return math.Round(value*factor) / factor
}

// deriveIntrinsicValuePerShare menghitung nilai intrinsik menggunakan Graham Number: sqrt(22.5 * EPS * BVPS)
func deriveIntrinsicValuePerShare(eps float64, bvps float64) float64 {
  if eps <= 0 || bvps <= 0 {
    return 0
  }
  return math.Sqrt(22.5 * eps * bvps)
}

func CalculateMedian(values []float64) float64 {
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

func CalculateMean(values []float64) float64 {
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

func ComputeStat(values []float64) StatSummary {
  return StatSummary{
    Median: CalculateMedian(values),
    Mean:   CalculateMean(values),
  }
}

func ParseVal(s string) float64 {
  clean := strings.ReplaceAll(s, ",", "")
  clean = strings.ReplaceAll(clean, " ", "")
  val, err := strconv.ParseFloat(clean, 64)
  if err != nil {
    return 0.0
  }
  return val
}

func FindAccountValue(m map[string]string, candidateKeys []string) float64 {
  for _, key := range candidateKeys {
    if valStr, exists := m[key]; exists {
      if val := ParseVal(valStr); val != 0 {
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
        if val := ParseVal(v); val != 0 {
          return val
        }
      }
    }
  }
  return 0.0
}
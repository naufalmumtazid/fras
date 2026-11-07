package infrastructure

import "time"

func GetDefaultStartPeriod() (int, int) {
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
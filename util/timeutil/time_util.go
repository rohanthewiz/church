package timeutil

import "time"

const (
	YY = "06" // short year
	YYYY = "2006"
	M = "1" // Std month
	MM = "01" // leading zero month
	MMM = "Jan" // short month
	MMMM = "January" // long month
	D = "2" // Std day
	DD = "02" // leading zero day
	DDD = "Mon" // short day
	DDDD = "Monday" // long day

	H = "3" // Std Hour
	HH = "15" // 24 Hour
	Mi = "4" // Std minute
	Min = "04" // leading zero minute
	S = "5" // short second
	SS = "05" // leading zero second

	PM = "PM"
	pm = "pm"
	TZ = "MST" // Std Timezone
	TZNum = "-0700"
	TZNumShort = "-07"
	TZNumColon = "-07:00"
	TZISO8601 = "Z0700"
	TZISO8601Colon = "Z07:00"
	ISO8601Date = "2006-01-02"
	ISO8601DateTime = "2006-01-02T15:04:05"
)

func CurrentYear() string {
	return time.Now().Format(YYYY)
}

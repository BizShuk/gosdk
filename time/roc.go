package time

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseROCDate parses a Republic of China (ROC) date string into a time.Time object.
// The input date string is expected to be in the format "YYYY/MM/DD", where YYYY is the ROC year.
// 100/08/07 => "2011/08/07".
// 100 => 2011/01/01
// 100/05 => 2011/05/01
func ParseROCDate(dateStr string) time.Time {
	parts := strings.Split(dateStr, "/")
	for len(parts) != 3 {
		parts = append(parts, "")
	}

	yearROC, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}
	}
	year := yearROC + 1911

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		month = 1
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil {
		day = 1
	}

	dateStrAD := fmt.Sprintf("%04d%02d%02d", year, month, day)
	parsedTime, _ := time.Parse("20060102", dateStrAD)
	return parsedTime
}

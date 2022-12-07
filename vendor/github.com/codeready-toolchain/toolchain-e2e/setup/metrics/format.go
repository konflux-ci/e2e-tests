package metrics

import "fmt"

const MB = 1 << 20

// bytesToMBString converts the given number of bytes to Megabytes and returns it as a string formatted to 2 decimal places
func bytesToMBString(bytes float64) string {
	return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
}

// simple returns the provided number as a string formatted to 4 decimal places
func simple(value float64) string {
	return fmt.Sprintf("%.4f", value)
}

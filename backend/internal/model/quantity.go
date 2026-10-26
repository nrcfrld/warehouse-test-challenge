package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var qtyPattern = regexp.MustCompile(`^[0-9]{1,15}(\.[0-9]{1,3})?$`)

// ParseQuantity parses a NUMERIC(18,3) value into exact thousandths.
func ParseQuantity(value string) (int64, error) {
	if !qtyPattern.MatchString(value) {
		return 0, Invalid("qty", "Use a positive decimal with up to 15 integer and 3 fractional digits")
	}
	parts := strings.SplitN(value, ".", 2)
	fraction := "000"
	if len(parts) == 2 {
		fraction = (parts[1] + "000")[:3]
	}
	n, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil {
		return 0, Invalid("qty", "Quantity is out of range")
	}
	return n, nil
}

// FormatQuantity formats exact thousandths without floating-point arithmetic.
func FormatQuantity(n int64) string { return fmt.Sprintf("%d.%03d", n/1000, n%1000) }

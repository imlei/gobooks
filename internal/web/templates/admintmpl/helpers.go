// 遵循产品需求 v1.0
package admintmpl

import "strconv"

// adminInt converts an int to string for use in templ expressions.
func adminInt(n int) string {
	return strconv.Itoa(n)
}

// adminUint converts a uint to string for use in templ expressions.
func adminUint(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

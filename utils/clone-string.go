package utils

import "strings"

func CloneString(s string) string {
	var b strings.Builder
	b.WriteString(s)
	return b.String()
}

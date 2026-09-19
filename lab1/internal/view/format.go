package view

import "strconv"

func formatWeight(weight float64) string {
	return strconv.FormatFloat(weight, 'f', 2, 64)
}

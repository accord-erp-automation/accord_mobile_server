package suplier

import (
	"fmt"
	"strings"
	"unicode"
)

func NormalizeName(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("Supplier ismi bo'sh bo'lmasligi kerak")
	}
	return trimmed, nil
}

func NormalizePhone(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("Telefon raqam bo'sh bo'lmasligi kerak")
	}

	var digits strings.Builder
	for _, r := range trimmed {
		if r == '+' {
			continue
		}
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("Telefon raqam faqat '+' va raqamlardan iborat bo'lishi kerak")
		}
		digits.WriteRune(r)
	}

	value := digits.String()
	if len(value) < 9 {
		return "", fmt.Errorf("Telefon raqam kamida 9 xonali bo'lishi kerak")
	}
	if len(value) > 12 {
		return "", fmt.Errorf("Telefon raqam 12 xonadan oshmasligi kerak")
	}

	return "+" + value, nil
}

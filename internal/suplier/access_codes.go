package suplier

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"
)

type AccessCredentials struct {
	Code   string
	Secret string
}

const supplierAccessPrefix = "10"

func GenerateAccessCredentials(supplier Supplier) (AccessCredentials, error) {
	normalizedName, err := NormalizeName(supplier.Name)
	if err != nil {
		return AccessCredentials{}, err
	}

	seed := strings.TrimSpace(supplier.Ref)
	if seed == "" {
		if phone := strings.TrimSpace(supplier.Phone); phone != "" {
			normalizedPhone, err := NormalizePhone(phone)
			if err == nil {
				seed = normalizedPhone
			}
		}
	}
	if seed == "" {
		seed = normalizedName
	}

	codeBody := hashToken(seed, 10)
	secretBody := hashToken(seed+"|"+normalizedName, 12)

	return AccessCredentials{
		Code:   supplierAccessPrefix + codeBody,
		Secret: secretBody,
	}, nil
}

func hashToken(value string, length int) string {
	sum := sha256.Sum256([]byte(value))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	encoded = strings.ToUpper(encoded)
	if length <= 0 || length >= len(encoded) {
		return encoded
	}
	return encoded[:length]
}

func SupplierAccessMessage(supplier Supplier) (string, error) {
	creds, err := GenerateAccessCredentials(supplier)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Supplier: %s\nTelefon: %s\nCode: %s",
		supplier.Name,
		strings.TrimSpace(supplier.Phone),
		creds.Code,
	), nil
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	RequestTimeout         time.Duration
	SettingsPassword       string
	AdminPassword          string
	DefaultTargetWarehouse string
	DefaultSourceWarehouse string
	DefaultUOM             string
	DefaultERPURL          string
	DefaultERPAPIKey       string
	DefaultERPAPISecret    string
	AdminkaPhone           string
	AdminkaName            string
	WerkaPhone             string
	WerkaName              string
	WerkaTelegramID        int64
}

func LoadFromEnv() (Config, error) {
	_ = godotenv.Load()

	timeout := 15 * time.Second
	if raw := os.Getenv("ERP_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, fmt.Errorf("invalid ERP_TIMEOUT_SECONDS: %q", raw)
		}
		timeout = time.Duration(seconds) * time.Second
	}

	var werkaTelegramID int64
	if raw := os.Getenv("WERKA_TELEGRAM_ID"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid WERKA_TELEGRAM_ID: %q", raw)
		}
		werkaTelegramID = value
	}

	return Config{
		RequestTimeout:         timeout,
		SettingsPassword:       os.Getenv("SETTINGS_PASSWORD"),
		AdminPassword:          os.Getenv("ADMIN_PASSWORD"),
		DefaultTargetWarehouse: os.Getenv("ERP_DEFAULT_TARGET_WAREHOUSE"),
		DefaultSourceWarehouse: os.Getenv("ERP_DEFAULT_SOURCE_WAREHOUSE"),
		DefaultUOM:             os.Getenv("ERP_DEFAULT_UOM"),
		DefaultERPURL:          os.Getenv("ERP_URL"),
		DefaultERPAPIKey:       os.Getenv("ERP_API_KEY"),
		DefaultERPAPISecret:    os.Getenv("ERP_API_SECRET"),
		AdminkaPhone:           os.Getenv("ADMINKA_PHONE"),
		AdminkaName:            os.Getenv("ADMINKA_NAME"),
		WerkaPhone:             os.Getenv("WERKA_PHONE"),
		WerkaName:              os.Getenv("WERKA_NAME"),
		WerkaTelegramID:        werkaTelegramID,
	}, nil
}

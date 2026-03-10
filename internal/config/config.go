package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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

func EnsureCoreRuntimeConfig(cfg *Config, envPath string, in io.Reader, out io.Writer) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	needsPrompt := strings.TrimSpace(cfg.DefaultERPURL) == "" ||
		strings.TrimSpace(cfg.DefaultERPAPIKey) == "" ||
		strings.TrimSpace(cfg.DefaultERPAPISecret) == ""
	if !needsPrompt {
		return nil
	}

	reader := bufio.NewReader(in)
	values := map[string]string{}
	if _, err := os.Stat(envPath); err == nil {
		existing, readErr := godotenv.Read(envPath)
		if readErr == nil {
			values = existing
		}
	}

	prompt := func(label, current string) (string, error) {
		if strings.TrimSpace(current) != "" {
			return strings.TrimSpace(current), nil
		}
		if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
			return "", err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return "", fmt.Errorf("%s is required", label)
		}
		return line, nil
	}

	var err error
	cfg.DefaultERPURL, err = prompt("ERP URL", cfg.DefaultERPURL)
	if err != nil {
		return err
	}
	cfg.DefaultERPAPIKey, err = prompt("ERP API key", cfg.DefaultERPAPIKey)
	if err != nil {
		return err
	}
	cfg.DefaultERPAPISecret, err = prompt("ERP API secret", cfg.DefaultERPAPISecret)
	if err != nil {
		return err
	}

	values["ERP_URL"] = cfg.DefaultERPURL
	values["ERP_API_KEY"] = cfg.DefaultERPAPIKey
	values["ERP_API_SECRET"] = cfg.DefaultERPAPISecret
	if strings.TrimSpace(values["ERP_TIMEOUT_SECONDS"]) == "" {
		values["ERP_TIMEOUT_SECONDS"] = strconv.Itoa(int(cfg.RequestTimeout / time.Second))
	}

	if err := godotenv.Write(values, envPath); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(out, "Core config saqlandi."); err != nil {
		return err
	}
	return nil
}

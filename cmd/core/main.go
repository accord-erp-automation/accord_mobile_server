package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mobile_server/internal/config"
	"mobile_server/internal/core"
	"mobile_server/internal/erpdb"
	"mobile_server/internal/erpnext"
	"mobile_server/internal/mobileapi"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("MOBILE_API_ADDR"))
	if addr == "" {
		addr = ":8081"
	}
	profileStorePath := strings.TrimSpace(os.Getenv("MOBILE_API_PROFILE_STORE_PATH"))
	if profileStorePath == "" {
		profileStorePath = "data/mobile_profile_prefs.json"
	}
	adminSupplierStorePath := strings.TrimSpace(os.Getenv("MOBILE_API_ADMIN_SUPPLIER_STORE_PATH"))
	if adminSupplierStorePath == "" {
		adminSupplierStorePath = "data/mobile_admin_suppliers.json"
	}
	sessionStorePath := strings.TrimSpace(os.Getenv("MOBILE_API_SESSION_STORE_PATH"))
	if sessionStorePath == "" {
		sessionStorePath = "data/mobile_sessions.json"
	}
	sessionTTL := 30 * 24 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("MOBILE_API_SESSION_TTL_HOURS")); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil {
			log.Fatalf("invalid MOBILE_API_SESSION_TTL_HOURS: %v", err)
		}
		if hours < 0 {
			log.Fatalf("invalid MOBILE_API_SESSION_TTL_HOURS: must be >= 0")
		}
		sessionTTL = time.Duration(hours) * time.Hour
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	erpClient := erpnext.NewClient(&http.Client{Timeout: cfg.RequestTimeout})
	service := core.NewERPAuthenticator(
		erpClient,
		cfg.DefaultERPURL,
		cfg.DefaultERPAPIKey,
		cfg.DefaultERPAPISecret,
		cfg.DefaultTargetWarehouse,
		os.Getenv("MOBILE_DEV_SUPPLIER_PREFIX"),
		os.Getenv("MOBILE_DEV_WERKA_PREFIX"),
		os.Getenv("MOBILE_DEV_WERKA_CODE"),
		cfg.WerkaPhone,
		os.Getenv("MOBILE_DEV_WERKA_NAME"),
		core.NewProfileStore(profileStorePath),
		core.NewAdminSupplierStore(adminSupplierStorePath),
	)
	service.SetAdminIdentity(
		"+998880000000",
		"Admin",
		"19621978",
		config.NewDotEnvPersister(".env"),
	)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ERP_DIRECT_READ_ENABLED")), "1") {
		siteConfigPath := strings.TrimSpace(os.Getenv("ERP_DIRECT_SITE_CONFIG_PATH"))
		if siteConfigPath != "" {
			dbCfg, err := erpdb.ConfigFromSiteConfig(siteConfigPath, cfg.DefaultTargetWarehouse)
			if err != nil {
				log.Printf("direct DB read disabled: %v", err)
			} else {
				dbCfg.Host = firstNonEmpty(strings.TrimSpace(os.Getenv("ERP_DIRECT_DB_HOST")), dbCfg.Host)
				dbCfg.Port = erpdb.ParsePort(strings.TrimSpace(os.Getenv("ERP_DIRECT_DB_PORT")), dbCfg.Port)
				dbCfg.User = firstNonEmpty(strings.TrimSpace(os.Getenv("ERP_DIRECT_DB_USER")), dbCfg.User)
				dbCfg.Password = firstNonEmpty(strings.TrimSpace(os.Getenv("ERP_DIRECT_DB_PASSWORD")), dbCfg.Password)
				dbCfg.Name = firstNonEmpty(strings.TrimSpace(os.Getenv("ERP_DIRECT_DB_NAME")), dbCfg.Name)
				reader, err := erpdb.Open(dbCfg)
				if err != nil {
					log.Printf("direct DB read disabled: %v", err)
				} else {
					service.SetDirectoryReader(reader)
					log.Printf("direct DB read enabled for Werka pickers via %s:%d/%s", dbCfg.Host, dbCfg.Port, dbCfg.Name)
				}
			}
		} else {
			log.Printf("direct DB read disabled: ERP_DIRECT_SITE_CONFIG_PATH is empty")
		}
	}

	server := mobileapi.NewServerWithSessionManager(
		service,
		mobileapi.NewPersistentSessionManager(sessionStorePath, sessionTTL),
	)
	warmupCtx, warmupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := service.WarmupWerkaCustomerIssue(warmupCtx); err != nil {
		log.Printf("werka customer issue warmup skipped: %v", err)
	} else {
		log.Printf("werka customer issue warmup ready")
	}
	warmupCancel()
	log.Printf("core listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("core stopped: %v", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

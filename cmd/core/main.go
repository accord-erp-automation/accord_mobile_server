package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"mobile_server/internal/config"
	"mobile_server/internal/core"
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
	)

	server := mobileapi.NewServer(service)
	log.Printf("core listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("core stopped: %v", err)
	}
}

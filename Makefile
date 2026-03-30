APP := ./cmd/core
ADDR ?= :8081
PID_FILE := .core.pid
LOG_FILE := .core.log
ENV_FILE := .env
REPO_ROOT := $(abspath ..)
PUBLIC_START_SCRIPT := $(REPO_ROOT)/mobile_app/start_domain_core.sh
ERP_DIRECT_SITE_CONFIG_PATH ?= /home/wikki/storage/local.git/erpnext_n1/erp/sites/erp.localhost/site_config.json
ERP_DIRECT_DB_HOST ?= 127.0.0.1
ERP_DIRECT_DB_PORT ?= 3306

.PHONY: run run-api run-local run-local-db stop test fmt tidy

run: stop
	@echo "Starting core with direct ERP DB read on $(ADDR)"
	@set -a; \
	if [ -f "$(ENV_FILE)" ]; then \
		. "./$(ENV_FILE)"; \
		echo "Loaded $(ENV_FILE)"; \
	fi; \
	set +a; \
	port="$(ADDR)"; \
	port="$${port##*:}"; \
	if ss -ltn "( sport = :$$port )" 2>/dev/null | tail -n +2 | grep -q .; then \
		echo "Port $$port is still busy; refusing to start"; \
		ss -ltnp "( sport = :$$port )" || true; \
		exit 1; \
	fi; \
	rm -f "$(LOG_FILE)"; \
	setsid env \
		ERP_DIRECT_READ_ENABLED=1 \
		ERP_DIRECT_SITE_CONFIG_PATH="$(ERP_DIRECT_SITE_CONFIG_PATH)" \
		ERP_DIRECT_DB_HOST="$(ERP_DIRECT_DB_HOST)" \
		ERP_DIRECT_DB_PORT="$(ERP_DIRECT_DB_PORT)" \
		MOBILE_API_ADDR="$(ADDR)" \
		go run $(APP) >"$(LOG_FILE)" 2>&1 < /dev/null & \
	echo $$! >"$(PID_FILE)"; \
	for _ in $$(seq 1 40); do \
		if curl -fsS "http://127.0.0.1:$$port/healthz" >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 0.5; \
	done; \
	if ! curl -fsS "http://127.0.0.1:$$port/healthz" >/dev/null 2>&1; then \
		echo "Core failed to start; see $(LOG_FILE)" >&2; \
		exit 1; \
	fi; \
	if [ -x "$(PUBLIC_START_SCRIPT)" ]; then \
		CORE_URL="http://127.0.0.1:$$port" BACKEND_ROOT="$(REPO_ROOT)" "$(PUBLIC_START_SCRIPT)"; \
	else \
		echo "Core ready at http://127.0.0.1:$$port"; \
	fi

run-api: stop
	@echo "Starting core on $(ADDR)"
	@set -a; \
	if [ -f "$(ENV_FILE)" ]; then \
		. "./$(ENV_FILE)"; \
		echo "Loaded $(ENV_FILE)"; \
	fi; \
	set +a; \
	port="$(ADDR)"; \
	port="$${port##*:}"; \
	if ss -ltn "( sport = :$$port )" 2>/dev/null | tail -n +2 | grep -q .; then \
		echo "Port $$port is still busy; refusing to start"; \
		ss -ltnp "( sport = :$$port )" || true; \
		exit 1; \
	fi; \
	rm -f "$(LOG_FILE)"; \
	setsid env MOBILE_API_ADDR="$(ADDR)" go run $(APP) >"$(LOG_FILE)" 2>&1 < /dev/null & \
	echo $$! >"$(PID_FILE)"; \
	for _ in $$(seq 1 40); do \
		if curl -fsS "http://127.0.0.1:$$port/healthz" >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 0.5; \
	done; \
	if ! curl -fsS "http://127.0.0.1:$$port/healthz" >/dev/null 2>&1; then \
		echo "Core failed to start; see $(LOG_FILE)" >&2; \
		exit 1; \
	fi; \
		if [ -x "$(PUBLIC_START_SCRIPT)" ]; then \
			CORE_URL="http://127.0.0.1:$$port" BACKEND_ROOT="$(REPO_ROOT)" "$(PUBLIC_START_SCRIPT)"; \
		else \
			echo "Core ready at http://127.0.0.1:$$port"; \
		fi

run-local: stop
	@echo "Starting core on $(ADDR)"
	@set -a; \
	if [ -f "$(ENV_FILE)" ]; then \
		. "./$(ENV_FILE)"; \
		echo "Loaded $(ENV_FILE)"; \
	fi; \
	set +a; \
	port="$(ADDR)"; \
	port="$${port##*:}"; \
	if ss -ltn "( sport = :$$port )" 2>/dev/null | tail -n +2 | grep -q .; then \
		echo "Port $$port is still busy; refusing to start"; \
		ss -ltnp "( sport = :$$port )" || true; \
		exit 1; \
	fi; \
	MOBILE_API_ADDR="$(ADDR)" go run $(APP)

run-local-db: stop
	@echo "Starting core with direct ERP DB read on $(ADDR)"
	@set -a; \
	if [ -f "$(ENV_FILE)" ]; then \
		. "./$(ENV_FILE)"; \
		echo "Loaded $(ENV_FILE)"; \
	fi; \
	set +a; \
	port="$(ADDR)"; \
	port="$${port##*:}"; \
	if ss -ltn "( sport = :$$port )" 2>/dev/null | tail -n +2 | grep -q .; then \
		echo "Port $$port is still busy; refusing to start"; \
		ss -ltnp "( sport = :$$port )" || true; \
		exit 1; \
	fi; \
	ERP_DIRECT_READ_ENABLED=1 \
	ERP_DIRECT_SITE_CONFIG_PATH="$(ERP_DIRECT_SITE_CONFIG_PATH)" \
	ERP_DIRECT_DB_HOST="$(ERP_DIRECT_DB_HOST)" \
	ERP_DIRECT_DB_PORT="$(ERP_DIRECT_DB_PORT)" \
	MOBILE_API_ADDR="$(ADDR)" \
	go run $(APP)

stop:
	@port="$(ADDR)"; \
	port="$${port##*:}"; \
	pids_file=""; \
	if [ -f "$(PID_FILE)" ]; then \
		pids_file="$$(cat "$(PID_FILE)" 2>/dev/null || true)"; \
	fi; \
	pids_port=$$(fuser "$$port/tcp" 2>/dev/null || true); \
	pids=$$(printf "%s\n%s\n" "$$pids_file" "$$pids_port" | tr ' ' '\n' | awk 'NF' | sort -u | paste -sd' ' -); \
	if [ -n "$$pids" ]; then \
		echo "Stopping process(es) on port $$port: $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		alive=$$(for pid in $$pids; do kill -0 $$pid 2>/dev/null && echo $$pid; done); \
		if [ -n "$$alive" ]; then \
			echo "Force killing process(es): $$alive"; \
			kill -9 $$alive 2>/dev/null || true; \
		fi; \
	else \
		echo "No running process found on port $$port"; \
	fi; \
	rm -f "$(PID_FILE)"

test:
	@go test ./...

fmt:
	@gofmt -w $$(find . -name '*.go' -type f)

tidy:
	@go mod tidy

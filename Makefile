APP := ./cmd/core
ADDR ?= :8081
PID_FILE := .core.pid
LOG_FILE := .core.log
ENV_FILE := .env

.PHONY: run stop test fmt tidy

run: stop
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

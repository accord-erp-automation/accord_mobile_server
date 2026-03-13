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
		. "$(ENV_FILE)"; \
		echo "Loaded $(ENV_FILE)"; \
	fi; \
	set +a; \
	MOBILE_API_ADDR="$(ADDR)" go run $(APP)

stop:
	@pids_file=""; \
	if [ -f "$(PID_FILE)" ]; then \
		pids_file="$$(cat "$(PID_FILE)" 2>/dev/null || true)"; \
	fi; \
	pids_go=$$(pgrep -x -f "go run ./cmd/core" || true); \
	pids_port=$$(lsof -t -iTCP:8081 -sTCP:LISTEN -n -P 2>/dev/null || true); \
	pids=$$(printf "%s\n%s\n%s\n" "$$pids_file" "$$pids_go" "$$pids_port" | tr ' ' '\n' | awk 'NF' | sort -u | paste -sd' ' -); \
	if [ -n "$$pids" ]; then \
		echo "Stopping core process(es): $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		alive=$$(for pid in $$pids; do kill -0 $$pid 2>/dev/null && echo $$pid; done); \
		if [ -n "$$alive" ]; then \
			echo "Force killing process(es): $$alive"; \
			kill -9 $$alive 2>/dev/null || true; \
		fi; \
	else \
		echo "No running core process found"; \
	fi; \
	rm -f "$(PID_FILE)"

test:
	@go test ./...

fmt:
	@gofmt -w $$(find . -name '*.go' -type f)

tidy:
	@go mod tidy

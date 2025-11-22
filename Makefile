GO ?= go
BIN_DIR ?= bin
GOFLAGS ?=
LDFLAGS ?=

AGENT_CMD := ./cmd/agent
MONITOR_CMD := ./cmd/monitor

AGENT_BIN := heartbeat-agent
MONITOR_BIN := heartbeat-monitor

.PHONY: build build-linux build-darwin clean

build: build-linux build-darwin

build-linux: $(BIN_DIR)/$(AGENT_BIN)-linux-amd64 $(BIN_DIR)/$(MONITOR_BIN)-linux-amd64 $(BIN_DIR)/$(AGENT_BIN)-linux-arm64 $(BIN_DIR)/$(MONITOR_BIN)-linux-arm64

build-darwin: $(BIN_DIR)/$(AGENT_BIN)-darwin-amd64 $(BIN_DIR)/$(MONITOR_BIN)-darwin-amd64 $(BIN_DIR)/$(AGENT_BIN)-darwin-arm64 $(BIN_DIR)/$(MONITOR_BIN)-darwin-arm64

$(BIN_DIR)/$(AGENT_BIN)-linux-amd64: $(AGENT_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-linux-amd64: $(MONITOR_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(AGENT_BIN)-linux-arm64: $(AGENT_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-linux-arm64: $(MONITOR_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(AGENT_BIN)-darwin-amd64: $(AGENT_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-darwin-amd64: $(MONITOR_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(AGENT_BIN)-darwin-arm64: $(AGENT_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-darwin-arm64: $(MONITOR_CMD)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

ldflags = $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)",)

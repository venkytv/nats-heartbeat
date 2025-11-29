GO ?= go
BIN_DIR ?= bin
GOFLAGS ?=
LDFLAGS ?=

AGENT_CMD := ./cmd/agent
MONITOR_CMD := ./cmd/monitor
STATUS_CMD := ./cmd/status

AGENT_BIN := heartbeat-agent
MONITOR_BIN := heartbeat-monitor
STATUS_BIN := heartbeat-status

GO_SOURCES := $(shell find cmd internal pkg -name '*.go')
BUILD_DEPS := $(GO_SOURCES) go.mod go.sum

.PHONY: build build-linux build-darwin clean

build: build-linux build-darwin

build-linux: $(BIN_DIR)/$(AGENT_BIN)-linux-amd64 $(BIN_DIR)/$(MONITOR_BIN)-linux-amd64 $(BIN_DIR)/$(STATUS_BIN)-linux-amd64 $(BIN_DIR)/$(AGENT_BIN)-linux-arm64 $(BIN_DIR)/$(MONITOR_BIN)-linux-arm64 $(BIN_DIR)/$(STATUS_BIN)-linux-arm64

build-darwin: $(BIN_DIR)/$(AGENT_BIN)-darwin-amd64 $(BIN_DIR)/$(MONITOR_BIN)-darwin-amd64 $(BIN_DIR)/$(STATUS_BIN)-darwin-amd64 $(BIN_DIR)/$(AGENT_BIN)-darwin-arm64 $(BIN_DIR)/$(MONITOR_BIN)-darwin-arm64 $(BIN_DIR)/$(STATUS_BIN)-darwin-arm64

$(BIN_DIR)/$(AGENT_BIN)-linux-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-linux-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(STATUS_BIN)-linux-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(STATUS_CMD)

$(BIN_DIR)/$(AGENT_BIN)-linux-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-linux-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(STATUS_BIN)-linux-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(STATUS_CMD)

$(BIN_DIR)/$(AGENT_BIN)-darwin-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-darwin-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(STATUS_BIN)-darwin-amd64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(STATUS_CMD)

$(BIN_DIR)/$(AGENT_BIN)-darwin-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(AGENT_CMD)

$(BIN_DIR)/$(MONITOR_BIN)-darwin-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(MONITOR_CMD)

$(BIN_DIR)/$(STATUS_BIN)-darwin-arm64: $(BUILD_DEPS)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) $(call ldflags) -o $@ $(STATUS_CMD)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

ldflags = $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)",)

# Minimal Makefile — all logic in ./scripts/*.sh
SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Convenience: if a script is missing or not executable, print guidance.
define run
	@if [[ ! -x "./scripts/$(1).sh" ]]; then \
		echo "scripts/$(1).sh not found or not executable. Create it and chmod +x."; \
		exit 1; \
	fi; \
	./scripts/$(1).sh $(2)
endef

.PHONY: help deps tidy fmt vet lint build prepare run test cover clean analytics ensure-valid

help:    ## Show targets
	@echo "Targets:" && printf "  %s\n" \
	  deps tidy fmt vet lint build prepare run test cover clean analytics

ensure-valid: tidy test lint vet examples

deps:    ## Download & tidy modules
	$(call run,deps)

tidy:    ## go mod tidy
	$(call run,tidy)

fmt:     ## gofmt (+ goimports if present)
	$(call run,fmt)

vet:     ## go vet
	$(call run,vet)

lint:    ## golangci-lint (fallback to vet)
	$(call run,lint)

build:   ## Build ./cmd -> ./bin/xmlui-mcp
	$(call run,build)

prepare: ## Prepare binary for use (remove quarantine)
	$(call run,prepare)

run:     ## Build then run; pass args like: make run -- --help
	$(call run,run,$(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)))

test:    ## Run all tests (unit + integration)
	$(call run,test)

cover:   ## Coverage report -> bin/coverage.html
	$(call run,cover)

clean:   ## Remove bin/
	$(call run,clean)

analytics: ## View analytics; usage: make analytics [summary|tools|searches|xmlui_search|xmlui_search_fail|xmlui_search_success|server]
	$(call run,analytics,$(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)))

# Allow passing args to `run` and `analytics` like: make run -- --help
%:
	@:

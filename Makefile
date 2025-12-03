.PHONY: gen gen-ts gen-go web api dev doctor tools diff check lock

OPENAPI_DIR   := openapi
WEB_OUT_TS    := apps/web/src/lib/api
GO_PINT_YML   := $(OPENAPI_DIR)/jp-pint.yaml
GO_AUDIT_YML  := $(OPENAPI_DIR)/audit-zip.yaml
GO_PINT_OUT   := apps/api/internal/pint/jp_pint.gen.go
GO_AUDIT_OUT  := apps/api/internal/auditzip/api.gen.go
AUDIT_TS_OUT  := $(WEB_OUT_TS)/audit-zip.types.ts
PINT_TS_OUT   := $(WEB_OUT_TS)/jp-pint.types.ts
OAPI_CODEGEN  := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
OPENAPI_DIFF  := npx -y openapi-diff

CHROMIUM_PATH := $(or $(PDF_CHROMIUM_PATH),$(shell find /home/node/.cache/ms-playwright -path "*/chrome-linux/chrome" -type f 2>/dev/null | head -1))

gen: gen-ts gen-go

gen-ts:
	npx -y openapi-typescript $(GO_PINT_YML) -o $(PINT_TS_OUT)
	npx -y openapi-typescript $(GO_AUDIT_YML) -o $(AUDIT_TS_OUT)

gen-go:
	$(OAPI_CODEGEN) -generate types,chi-server -package pint -o $(GO_PINT_OUT) $(GO_PINT_YML)
	$(OAPI_CODEGEN) -generate types,chi-server -package auditzip -o $(GO_AUDIT_OUT) $(GO_AUDIT_YML)

diff:
	$(OPENAPI_DIFF) $(GO_PINT_YML) $(OPENAPI_DIR)/jp-pint.lock.yaml || true
	$(OPENAPI_DIFF) $(GO_AUDIT_YML) $(OPENAPI_DIR)/audit-zip.lock.yaml || true

check: diff
	git diff --exit-code $(GO_PINT_YML) $(GO_AUDIT_YML)

lock:
	cp $(GO_PINT_YML) $(OPENAPI_DIR)/jp-pint.lock.yaml
	cp $(GO_AUDIT_YML) $(OPENAPI_DIR)/audit-zip.lock.yaml

web:
	# ワークスペースの pnpm (v10 系) を使用
	pnpm --filter web dev

api:
	cd apps/api && PDF_CHROMIUM_PATH="$(CHROMIUM_PATH)" air

dev:
	$(MAKE) -j2 web api

doctor:
	@echo "Chromium path: $(CHROMIUM_PATH)"
	node -v || true
	pnpm -v || true
	go version || true
	$(OAPI_CODEGEN) -version || true

tools:
	# air が無い場合にインストール（Go の bin PATH が通っていることを想定）
	command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@v1.52.3

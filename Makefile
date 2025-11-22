# PINT_OPENAPI=openapi/jp-pint.yaml
# PINT_GO_OUT=apps/api/internal/pint/api.gen.go
# PINT_TS_OUT=apps/web/src/lib/api/jp-pint.types.ts

# AUDIT_OPENAPI=openapi/audit-zip.yaml
# AUDIT_GO_OUT=apps/api/internal/auditzip/api.gen.go
# AUDIT_TS_OUT=apps/web/src/lib/api/audit-zip.types.ts

# .PHONY: gen go-gen ts-gen diff check

# gen: go-gen ts-gen

# go-gen:
# 	oapi-codegen -generate types,chi-server -o $(PINT_GO_OUT) -package pint $(PINT_OPENAPI)
# 	oapi-codegen -generate types,chi-server -o $(AUDIT_GO_OUT) -package auditzip $(AUDIT_OPENAPI)

# ts-gen:
# 	openapi-typescript $(PINT_OPENAPI) -o $(PINT_TS_OUT)
# 	openapi-typescript $(AUDIT_OPENAPI) -o $(AUDIT_TS_OUT)

# diff:
# 	openapi-diff $(PINT_OPENAPI) openapi/jp-pint.lock.yaml
# 	openapi-diff $(AUDIT_OPENAPI) openapi/audit-zip.lock.yaml

# check: diff
# 	git diff --exit-code $(PINT_OPENAPI) $(AUDIT_OPENAPI)



.PHONY: gen gen-ts gen-go web api dev doctor tools

OPENAPI_DIR := openapi
WEB_OUT_TS  := apps/web/src/lib/api
GO_PINT_YML := $(OPENAPI_DIR)/jp-pint.yaml
GO_AUDIT_YML:= $(OPENAPI_DIR)/audit-zip.yaml
OAPI_CODEGEN := go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.2.1

gen: gen-ts gen-go

gen-ts:
	# TypeScript 型生成（pnpm不要）
	npx -y openapi-typescript $(GO_PINT_YML)  -o $(WEB_OUT_TS)/jp-pint.types.ts
	npx -y openapi-typescript $(GO_AUDIT_YML) -o $(WEB_OUT_TS)/audit-zip.types.ts

gen-go:
	$(OAPI_CODEGEN) -generate types,chi-server -package pint \
		-o apps/api/internal/pint/jp_pint.gen.go $(GO_PINT_YML)
	$(OAPI_CODEGEN) -generate types,chi-server -package auditzip \
		-o apps/api/internal/auditzip/audit_zip.gen.go $(GO_AUDIT_YML)

web:
	# ワークスペースの pnpm (v10 系) を使用
	pnpm --filter apps/web dev

api:
	cd apps/api && air

dev:
	$(MAKE) -j2 web api

doctor:
	node -v && pnpm -v && go version && $(OAPI_CODEGEN) -version || true

tools:
	# air が無い場合にインストール（Go の bin PATH が通っていることを想定）
	command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@v1.52.3



# .PHONY: gen gen-ts gen-go lock diff check web api dev doctor

# OPENAPI_DIR := openapi
# PINT_YML    := $(OPENAPI_DIR)/jp-pint.yaml
# AUDIT_YML   := $(OPENAPI_DIR)/audit-zip.yaml

# WEB_TS_DIR     := apps/web/src/lib/api
# GO_PINT_OUT    := apps/api/internal/pint/jp_pint.gen.go
# GO_AUDIT_OUT   := apps/api/internal/auditzip/audit_zip.gen.go

# gen: gen-ts gen-go

# gen-ts:
# 	# TS 型（openapi-typescript は devDeps 前提、pnpm exec で呼ぶ）
# 	pnpm exec openapi-typescript $(PINT_YML)  -o $(WEB_TS_DIR)/jp-pint.types.ts
# 	pnpm exec openapi-typescript $(AUDIT_YML) -o $(WEB_TS_DIR)/audit-zip.types.ts

# gen-go:
# 	# Go 型+chiサーバスタブ
# 	oapi-codegen -generate types,chi-server -package pint     -o $(GO_PINT_OUT)  $(PINT_YML)
# 	oapi-codegen -generate types,chi-server -package auditzip  -o $(GO_AUDIT_OUT) $(AUDIT_YML)

# # いまの契約をベースラインとしてロック
# lock:
# 	cp $(PINT_YML)  $(OPENAPI_DIR)/jp-pint.lock.yaml
# 	cp $(AUDIT_YML) $(OPENAPI_DIR)/audit-zip.lock.yaml

# # 契約差分（ライブラリは dlx で都度実行。常時入れたいなら devDeps に追加）
# diff:
# 	pnpm dlx openapi-diff $(PINT_YML)  $(OPENAPI_DIR)/jp-pint.lock.yaml  || true
# 	pnpm dlx openapi-diff $(AUDIT_YML) $(OPENAPI_DIR)/audit-zip.lock.yaml || true

# # CI で差分があれば落とす
# check: diff
# 	git diff --exit-code $(PINT_YML) $(AUDIT_YML)

# web:
# 	pnpm --filter apps/web dev

# api:
# 	cd apps/api && air

# dev:
# 	$(MAKE) -j2 web api

# doctor:
# 	node -v && pnpm -v && go version && oapi-codegen -version || true

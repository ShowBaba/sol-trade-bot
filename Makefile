APP_CMD=./cmd/solbot

all: tidy run

tidy:
	go mod tidy

run:
	go run $(APP_CMD)

.PHONY: web-dev
web-dev:
	cd web && npm run dev


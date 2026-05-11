SHELL := /bin/bash
ENV_FILE ?= .env

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export
endif

.PHONY: run build tidy migrate-up migrate-down migrate-create onlyoffice-up onlyoffice-down backup

run:
	go run ./cmd/server

build:
	go build -o ./bin/server ./cmd/server

tidy:
	go mod tidy

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $$name

onlyoffice-up:
	docker run -d --name onlyoffice -p 8082:80 \
		-e JWT_ENABLED=false \
		-e ALLOW_PRIVATE_IP_ADDRESS=true \
		-e ALLOW_META_IP_ADDRESS=true \
		--restart=unless-stopped onlyoffice/documentserver

onlyoffice-down:
	docker stop onlyoffice && docker rm onlyoffice

backup:
	./scripts/backup.sh

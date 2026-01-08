-include .env
export

.PHONY: up run build clean stop migrate-up migrate-down migrate-create

up:
	docker-compose --env-file .env -f docker/docker-compose.yml up -d

stop:
	docker-compose --env-file .env -f docker/docker-compose.yml stop

run:
	go run .

build:
	go build -o bin/commission-tracker .

clean:
	rm -f bin/commission-tracker
	rm -rf bin/

migrate-up:
	goose -dir ./sql/schema postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5433/${POSTGRES_DB}?sslmode=disable" up

migrate-down:
	goose -dir ./sql/schema postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5433/${POSTGRES_DB}?sslmode=disable" down

migrate-create:
	@test -n "$(name)" || (echo "name is required"; exit 1)
	goose -dir ./sql/schema create $(name) sql

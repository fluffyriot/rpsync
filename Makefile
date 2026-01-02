.PHONY: run

up:
	docker-compose -f docker/docker-compose.yml up -d

run:
	@if [ -f .env ]; then . ./.env; fi; go run ./cmd

# You might also want a build target for creating a binary
build:
	go build -o bin/commission-tracker ./cmd

# And a clean target
clean:
	rm -f bin/commission-tracker
	rm -rf bin/

stop:
	docker-compose -f docker/docker-compose.yml stop
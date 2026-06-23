.PHONY: up down logs api-shell web-shell test

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f api web postgres

api-shell:
	docker compose run --rm api sh

web-shell:
	docker compose run --rm web sh

test:
	docker compose run --rm api go test ./...

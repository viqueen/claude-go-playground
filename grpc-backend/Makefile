.PHONY: codegen tidy vet build test infra start debug teardown clean

# --- codegen ---

codegen:
	docker build --target generate -t codegen .
	docker run --rm -v ./gen:/out/gen codegen cp -r /build/gen/. /out/gen/

tidy: codegen
	go mod tidy

# --- checks ---

vet: tidy
	go vet ./...

build: vet
	docker build .

test: vet
	go test ./...

# --- infra ---

infra:
	docker compose up -d
	@echo "waiting for postgres to be healthy..."
	@until docker compose exec -T postgres pg_isready -U playground > /dev/null 2>&1; do sleep 1; done
	@echo "infra is up"

# --- run ---

start: infra
	go tool air

debug: infra
	go tool air -c .air.debug.toml

# --- teardown ---

teardown:
	docker compose down

clean: teardown
	rm -rf gen/ tmp/

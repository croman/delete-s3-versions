full: lint test build

lint:
	@revive -config .revive.toml -formatter stylish ./...

test:
	@richgo test -cover -coverprofile cover.out ./...

build:
	@go build ./...

cover:
	@go tool cover -html=cover.out

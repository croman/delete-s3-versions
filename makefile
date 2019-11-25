full: lint test

lint:
	@revive -config .revive.toml -formatter stylish ./...

test:
	@richgo test -cover -coverprofile cover.out ./...

cover:
	@go tool cover -html=cover.out

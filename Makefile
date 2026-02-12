.PHONY: build clean test coverage
build:
	mkdir -p bin
	go build -buildvcs=false -o bin/stet ./cli/cmd/stet
	GOOS=linux GOARCH=amd64 go build -buildvcs=false -o bin/stet-linux-amd64 ./cli/cmd/stet
	GOOS=darwin GOARCH=amd64 go build -buildvcs=false -o bin/stet-darwin-amd64 ./cli/cmd/stet

clean:
	rm -f bin/stet bin/stet-linux-amd64 bin/stet-darwin-amd64 coverage.out

test:
	go test ./cli/... -count=1

coverage: coverage.out
	@bash scripts/check-coverage.sh coverage.out

coverage.out:
	go test ./cli/... -coverprofile=coverage.out -count=1

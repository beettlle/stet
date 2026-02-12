.PHONY: build clean
build:
	mkdir -p bin
	go build -o bin/stet ./cli/cmd/stet
	GOOS=linux GOARCH=amd64 go build -o bin/stet-linux-amd64 ./cli/cmd/stet
	GOOS=darwin GOARCH=amd64 go build -o bin/stet-darwin-amd64 ./cli/cmd/stet

clean:
	rm -f bin/stet bin/stet-linux-amd64 bin/stet-darwin-amd64

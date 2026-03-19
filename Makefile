.PHONY: build run test clean

build:
	go build -o bin/uniapi ./cmd/uniapi

run: build
	./bin/uniapi

test:
	go test ./... -v -race

clean:
	rm -rf bin/

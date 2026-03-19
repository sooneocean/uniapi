.PHONY: build run test clean frontend build-linux docker

frontend:
	cd frontend && npm run build
	rm -rf internal/web/dist
	cp -r frontend/dist internal/web/dist

build: frontend
	go build -o bin/uniapi ./cmd/uniapi

run: build
	./bin/uniapi

test:
	go test ./... -v -race

build-linux: frontend
	GOOS=linux GOARCH=amd64 go build -o bin/uniapi-linux-amd64 ./cmd/uniapi
	GOOS=linux GOARCH=arm64 go build -o bin/uniapi-linux-arm64 ./cmd/uniapi

docker:
	docker build -t uniapi/uniapi .

clean:
	rm -rf bin/ internal/web/dist

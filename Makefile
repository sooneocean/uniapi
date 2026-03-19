.PHONY: build run test clean frontend

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

clean:
	rm -rf bin/ internal/web/dist

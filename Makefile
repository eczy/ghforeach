build:
	go build -o ./out/ghforeach ./cmd/main.go 
	chmod +x ./out/ghforeach

test:
	go test ./...

build:
	go build -o ./out/ghforeach ./cmd/main.go 
	chmod +x ./out/ghforeach

test:
	go test ./...

test/integration:
	GHFOREACH_ENABLE_INTEGRATION_TEST=1 go test ./...

install: build
	cp ./out/ghforeach /usr/local/bin/ghforeach

uninstall:
	rm /usr/local/bin/ghforeach

clean:
	rm -rf ./out
	

build:
	go build -o $(GOPATH)/bin/pchain ./cmd/

clean:
	rm $(GOPATH)/bin/pchain

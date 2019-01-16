
build:
	go build -o $(GOPATH)/bin/pchain ./cmd/

clean:
	rm $(GOPATH)/bin/pchain

pchain:
	build/env.sh
	@echo "Done building."
	@echo "Run ./bin/pchain to launch pchain network."

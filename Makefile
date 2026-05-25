VERSION ?= dev
BIN_DIR ?= bin
DIST_DIR ?= dist

.PHONY: test build dist release clean

test:
	go test ./...

build: $(BIN_DIR)/agentcall

$(BIN_DIR)/agentcall:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/agentcall ./cmd/agentcall

dist:
	go run ./cmd/agentcall-dist --version $(VERSION) --output $(DIST_DIR)

release: dist
	git rev-parse --verify $(VERSION) >/dev/null 2>&1 || git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	gh release create $(VERSION) $(DIST_DIR)/* --verify-tag --title $(VERSION) --generate-notes

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

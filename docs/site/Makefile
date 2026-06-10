.PHONY: install build clean help

ZOLA_VERSION = v0.21.0
ZOLA_URL = https://github.com/getzola/zola/releases/download/$(ZOLA_VERSION)/zola-$(ZOLA_VERSION)-x86_64-unknown-linux-gnu.tar.gz
ZOLA_BIN = ./zola

build: install
	@$(ZOLA_BIN) build --minify

install:
	@echo "Installing Zola $(ZOLA_VERSION)..."
	@curl -L $(ZOLA_URL) -o /tmp/zola.tar.gz
	@tar xzf /tmp/zola.tar.gz -C .
	@rm /tmp/zola.tar.gz
	@chmod +x $(ZOLA_BIN)
	@echo "Zola installed"
	@$(ZOLA_BIN) --version

clean:
	@rm -rf public

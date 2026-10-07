GO ?= $(CURDIR)/.tools/go/bin/go
export PATH := $(CURDIR)/.tools/go/bin:$(PATH)
export GOPATH := $(CURDIR)/.tools/gopath
export GOCACHE := $(CURDIR)/.tools/gocache
export GOEXPERIMENT := nojsonv2
export GOPRIVATE := github.com/cztamas/qwacamole,$(GOPRIVATE)
export XDG_DATA_HOME := $(CURDIR)/.local/xdg-data
export XDG_CONFIG_HOME := $(CURDIR)/.local/xdg-config
STATE ?= .local
BROWSER_ARGS ?=
DEMO_ARGS ?=
.PHONY: bootstrap bootstrap-firefox build browser-demo firefox-demo firefox-check firefox-test demo test browser-test browser-check integration-test fixture clean clean-all
bootstrap:
	./scripts/bootstrap.sh
build: bootstrap
	cd devtools && $(GO) build -o ../bin/qwacamole ./cmd/qwacamole
	GOWORK=off .tools/bin/xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@v0.0.0=$(CURDIR)/caddy --with github.com/cztamas/qwacamole/devtools@v0.0.0=$(CURDIR)/devtools --replace github.com/cztamas/qwacamole=$(CURDIR) --output bin/caddy > .local/evidence/xcaddy-build.log 2>&1
demo: build
	bin/qwacamole demo -state $(STATE)
browser-demo: build
	bin/qwacamole browser-demo -state $(STATE) $(DEMO_ARGS) $(BROWSER_ARGS)
bootstrap-firefox:
	./scripts/bootstrap-firefox.sh
firefox-demo: bootstrap-firefox build
	bin/qwacamole firefox-demo -state $(STATE) $(DEMO_ARGS) $(BROWSER_ARGS)
firefox-check:
	bin/qwacamole firefox-check -state $(STATE) $(BROWSER_ARGS)
# Requires a Firefox demo state and its fixture server; see docs/firefox.md.
firefox-test:
	bin/qwacamole firefox-suite -state $(STATE) $(BROWSER_ARGS)
.PHONY: firefox-integration
firefox-integration: bootstrap-firefox build
	bin/qwacamole firefox-integration -state $(STATE) $(BROWSER_ARGS)
test:
	$(GO) test -race ./...
	cd agent && $(GO) test -race ./...
	cd caddy && $(GO) test -race ./...
	cd devtools && $(GO) test -race ./...
	./scripts/check-boundaries.sh
# Full browser acceptance remains nonzero while stock Chrome ignores exp.
browser-test: build
	bin/qwacamole integration-test -state $(STATE) $(BROWSER_ARGS)
integration-test: browser-test
browser-check:
	bin/qwacamole browser -state $(STATE) -ct-exception -tls-root $(STATE)/pki/pebble-root.pem $(BROWSER_ARGS)
fixture:
	bin/qwacamole fixture -state $(STATE)
# Stop this project's running demo with Ctrl-C before deleting state.
clean:
	bin/qwacamole clean -state $(STATE)
clean-all: clean
	rm -rf bin .tools

.PHONY: plugin-build
plugin-build: bootstrap
	GOWORK=off .tools/bin/xcaddy build v2.10.2 --with github.com/cztamas/qwacamole/caddy@v0.0.0=$(CURDIR)/caddy --replace github.com/cztamas/qwacamole=$(CURDIR) --output bin/caddy-plugin > .local/evidence/xcaddy-plugin-build.log 2>&1

.PHONY: plugin-smoke remote-install-check
plugin-smoke: plugin-build
	cd devtools && $(GO) build -o ../bin/qwacamole ./cmd/qwacamole
	bin/qwacamole plugin-smoke -state $(STATE)
remote-install-check:
	./scripts/check-remote-install.sh $(REF)

.PHONY: agent-build bootstrap-nginx nginx-smoke
agent-build: bootstrap
	cd agent && $(GO) build -o ../bin/qwacamole-agent ./cmd/qwacamole-agent
bootstrap-nginx:
	./scripts/bootstrap-nginx.sh
nginx-smoke: agent-build
	cd devtools && $(GO) build -o ../bin/qwacamole ./cmd/qwacamole
	bin/qwacamole nginx-smoke -state $(STATE)

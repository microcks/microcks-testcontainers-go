.PHONY: test
test:
	$(MAKE) test-microcks

.PHONY: dependencies-scan
dependencies-scan:
	@echo ">> Scanning dependencies in $(CURDIR)..."
	go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth --skip-update-check

.PHONY: test-%
test-%:
	@echo "Running $* tests..."
	gotestsum \
		--format short-verbose \
		--rerun-fails=1 \
		--packages="./..." \
		--junitfile TEST-$*.xml \
		-- \
		-timeout=2m

.PHONY: tools
tools:
	go mod download

.PHONY: tools-tidy
tools-tidy:
	go mod tidy
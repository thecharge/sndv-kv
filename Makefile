.PHONY: all test benchmark profile compare dashboard clean

all: test benchmark
# go env -w CGO_ENABLED=1

test:
	go test ./... -v -cover

test-coverage:
	go env -w CGO_ENABLED=1
	./scripts/test_coverage.sh

test-race:
	go test ./... -race

benchmark:
	python3 scripts/bench_all.py

profile:
	./scripts/profile.sh

compare:
	python3 scripts/compare_versions.py

dashboard:
	python3 scripts/generate_dashboard.py

human:
	python3 scripts/human_latency.py

ci: test-coverage benchmark compare
	@echo "✅ CI complete"

clean:
	rm -rf benchmark_reports coverage.out *.prof sndv-kv-bench
fmt: 
	go fmt ./...
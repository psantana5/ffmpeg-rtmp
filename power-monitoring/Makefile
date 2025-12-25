.PHONY: help up up-build down restart ps logs targets prom-reload grafana test-suite test-single test-multi test-batch analyze nvidia-up nvidia-up-build

COMPOSE ?= docker compose
PYTHON ?= python3

SERVICE ?=

NAME ?= quick
BITRATE ?= 1000k
RESOLUTION ?= 1280x720
FPS ?= 30
DURATION ?= 60
STABILIZATION ?= 10
COOLDOWN ?= 10

COUNT ?= 4

help:
	@echo "Power Monitoring - common commands"
	@echo ""
	@echo "Stack"
	@echo "  make up              Start stack"
	@echo "  make up-build        Build + start stack"
	@echo "  make down            Stop stack"
	@echo "  make restart         Restart stack"
	@echo "  make ps              Show container status"
	@echo "  make logs SERVICE=prometheus   Tail logs for a service"
	@echo ""
	@echo "GPU (NVIDIA)"
	@echo "  make nvidia-up       Start stack with NVIDIA profile"
	@echo "  make nvidia-up-build Build + start stack with NVIDIA profile"
	@echo ""
	@echo "Prometheus/Grafana"
	@echo "  make targets         Print Prometheus scrape targets"
	@echo "  make prom-reload     Reload Prometheus config"
	@echo ""
	@echo "Tests"
	@echo "  make test-suite      Run default test suite"
	@echo "  make test-batch      Run stress-matrix batch (batch_stress_matrix.json)"
	@echo "  make analyze         Analyze latest test results (and export CSV)"
	@echo ""
	@echo "Examples"
	@echo "  make logs SERVICE=results-exporter"
	@echo "  make test-single NAME=quick BITRATE=1000k DURATION=60"
	@echo ""

up:
	$(COMPOSE) up -d

up-build:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart

ps:
	$(COMPOSE) ps

logs:
	@test -n "$(SERVICE)" || (echo "SERVICE is required. Example: make logs SERVICE=prometheus" && exit 2)
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

targets:
	$(PYTHON) - <<'PY'
import json, urllib.request
url='http://localhost:9090/api/v1/targets'
with urllib.request.urlopen(url, timeout=10) as r:
    data=json.load(r)
active=data.get('data',{}).get('activeTargets',[])
print('Active targets:', len(active))
for t in active:
    labels=t.get('labels',{})
    job=labels.get('job','?')
    inst=labels.get('instance','?')
    health=t.get('health','?')
    print(f"- job={job:16s} instance={inst:24s} health={health}")
PY

prom-reload:
	curl -fsS -X POST http://localhost:9090/-/reload >/dev/null

nvidia-up:
	$(COMPOSE) --profile nvidia up -d

nvidia-up-build:
	$(COMPOSE) --profile nvidia up -d --build

test-suite:
	$(PYTHON) run_tests.py suite

test-single:
	$(PYTHON) run_tests.py --output-dir ./test_results single --name $(NAME) --bitrate $(BITRATE) --resolution $(RESOLUTION) --fps $(FPS) --duration $(DURATION) --stabilization $(STABILIZATION) --cooldown $(COOLDOWN)

test-multi:
	$(PYTHON) run_tests.py --output-dir ./test_results multi --count $(COUNT) --bitrate $(BITRATE) --resolution $(RESOLUTION) --fps $(FPS) --duration $(DURATION) --stabilization $(STABILIZATION) --cooldown $(COOLDOWN)

test-batch:
	$(PYTHON) run_tests.py --output-dir ./test_results batch --file batch_stress_matrix.json

analyze:
	$(PYTHON) analyze_results.py

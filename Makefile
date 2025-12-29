.PHONY: help up up-build down restart ps logs targets prom-reload grafana test-suite test-single test-multi test-batch analyze nvidia-up nvidia-up-build lint format test pre-commit

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
	@echo "  make prom-reload     Reload Prometheus config"
	@echo ""
	@echo "Tests"
	@echo "  make test-suite      Run default test suite"
	@echo "  make test-batch      Run stress-matrix batch (batch_stress_matrix.json)"
	@echo "  make analyze         Analyze latest test results (and export CSV)"
	@echo "  make retrain-models  Retrain ML models from test results"
	@echo ""
	@echo "Development"
	@echo "  make lint            Run ruff checks"
	@echo "  make format          Run ruff formatter"
	@echo "  make test            Run pytest"
	@echo "  make pre-commit      Run pre-commit on all files"
	@echo ""
	@echo "Examples"
	@echo "  make logs SERVICE=results-exporter"
	@echo "  make test-single NAME=quick BITRATE=1000k DURATION=60"
	@echo ""

up:
	@mkdir -p test_results
	$(COMPOSE) up -d

up-build:
	@mkdir -p test_results
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

prom-reload:
	curl -fsS -X POST http://localhost:9090/-/reload >/dev/null

nvidia-up:
	@mkdir -p test_results
	$(COMPOSE) --profile nvidia up -d

nvidia-up-build:
	@mkdir -p test_results
	$(COMPOSE) --profile nvidia up -d --build

test-suite:
	$(PYTHON) scripts/run_tests.py suite

test-single:
	$(PYTHON) scripts/run_tests.py --output-dir ./test_results single --name $(NAME) --bitrate $(BITRATE) --resolution $(RESOLUTION) --fps $(FPS) --duration $(DURATION) --stabilization $(STABILIZATION) --cooldown $(COOLDOWN)

test-multi:
	$(PYTHON) scripts/run_tests.py --output-dir ./test_results multi --count $(COUNT) --bitrate $(BITRATE) --resolution $(RESOLUTION) --fps $(FPS) --duration $(DURATION) --stabilization $(STABILIZATION) --cooldown $(COOLDOWN)

test-batch:
	$(PYTHON) scripts/run_tests.py --output-dir ./test_results batch --file batch_stress_matrix.json

analyze:
	$(PYTHON) scripts/analyze_results.py

retrain-models:
	$(PYTHON) scripts/retrain_models.py --results-dir ./test_results --models-dir ./models

lint:
	$(PYTHON) -m ruff check .

format:
	$(PYTHON) -m ruff format .

test:
	$(PYTHON) -m pytest

pre-commit:
	$(PYTHON) -m pre_commit run --all-files

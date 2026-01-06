#!/usr/bin/env python3
"""
Update Production Monitoring Dashboard with Improved Queries
This script generates an optimized Grafana dashboard for FFmpeg-RTMP monitoring
"""

import json

def create_improved_dashboard():
    """Create comprehensive production monitoring dashboard"""
    
    dashboard = {
        "annotations": {"list": []},
        "editable": True,
        "fiscalYearStartMonth": 0,
        "graphTooltip": 1,
        "id": None,
        "links": [],
        "liveNow": False,
        "panels": [],
        "refresh": "10s",
        "schemaVersion": 38,
        "style": "dark",
        "tags": ["production", "monitoring", "ffmpeg-rtmp"],
        "templating": {"list": []},
        "time": {"from": "now-1h", "to": "now"},
        "timepicker": {
            "refresh_intervals": ["5s", "10s", "30s", "1m", "5m", "15m"]
        },
        "timezone": "",
        "title": "FFmpeg-RTMP Production Monitoring",
        "uid": "ffmpeg-rtmp-production",
        "version": 0,
        "weekStart": ""
    }
    
    panels = []
    
    # Row 1: Key Metrics
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "mappings": [],
                "max": 100,
                "min": 0,
                "thresholds": {
                    "mode": "absolute",
                    "steps": [
                        {"color": "red", "value": None},
                        {"color": "yellow", "value": 95},
                        {"color": "green", "value": 99}
                    ]
                },
                "unit": "percent"
            }
        },
        "gridPos": {"h": 8, "w": 6, "x": 0, "y": 0},
        "id": 1,
        "options": {
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "showThresholdLabels": False,
            "showThresholdMarkers": True
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "100 * sum(ffrtmp_jobs_total{state=\"completed\"}) / (sum(ffrtmp_jobs_total{state=\"completed\"}) + sum(ffrtmp_jobs_total{state=\"failed\"}))",
            "refId": "A"
        }],
        "title": "Job Success Rate (All Time)",
        "type": "gauge"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "mappings": [],
                "max": 100,
                "min": 0,
                "thresholds": {
                    "mode": "absolute",
                    "steps": [
                        {"color": "red", "value": None},
                        {"color": "yellow", "value": 80},
                        {"color": "green", "value": 95}
                    ]
                },
                "unit": "percent"
            }
        },
        "gridPos": {"h": 8, "w": 6, "x": 6, "y": 0},
        "id": 2,
        "options": {
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "showThresholdLabels": False,
            "showThresholdMarkers": True
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "100 * (sum(increase(ffrtmp_jobs_total{state=\"completed\"}[1h])) / (sum(increase(ffrtmp_jobs_total[1h])) + 1))",
            "refId": "A"
        }],
        "title": "Success Rate (Last Hour)",
        "type": "gauge"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "thresholds"},
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [
                        {"color": "green", "value": None},
                        {"color": "yellow", "value": 50},
                        {"color": "red", "value": 100}
                    ]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 4, "w": 3, "x": 12, "y": 0},
        "id": 3,
        "options": {
            "colorMode": "value",
            "graphMode": "area",
            "justifyMode": "auto",
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "textMode": "auto"
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_active_jobs",
            "refId": "A"
        }],
        "title": "Active Jobs",
        "type": "stat"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "thresholds"},
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [
                        {"color": "green", "value": None},
                        {"color": "yellow", "value": 10000},
                        {"color": "red", "value": 50000}
                    ]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 4, "w": 3, "x": 15, "y": 0},
        "id": 4,
        "options": {
            "colorMode": "value",
            "graphMode": "area",
            "justifyMode": "auto",
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "textMode": "auto"
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_queue_length",
            "refId": "A"
        }],
        "title": "Queue Length",
        "type": "stat"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "thresholds"},
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 4, "w": 3, "x": 18, "y": 0},
        "id": 5,
        "options": {
            "colorMode": "value",
            "graphMode": "area",
            "justifyMode": "auto",
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "textMode": "auto"
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_nodes_total",
            "refId": "A"
        }],
        "title": "Worker Nodes",
        "type": "stat"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "thresholds"},
                "decimals": 1,
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "s"
            }
        },
        "gridPos": {"h": 4, "w": 3, "x": 21, "y": 0},
        "id": 6,
        "options": {
            "colorMode": "value",
            "graphMode": "none",
            "justifyMode": "auto",
            "orientation": "auto",
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "textMode": "auto"
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_job_duration_seconds",
            "refId": "A"
        }],
        "title": "Avg Job Duration",
        "type": "stat"
    })
    
    # Row 2: Job Stats Time Series
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "line",
                    "fillOpacity": 10,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "linear",
                    "lineWidth": 2,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": False,
                    "stacking": {"group": "A", "mode": "none"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 4},
        "id": 7,
        "options": {
            "legend": {
                "calcs": ["last", "max"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "none"}
        },
        "targets": [
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "sum(ffrtmp_jobs_total{state=\"completed\"})",
                "legendFormat": "Completed",
                "refId": "A"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "sum(ffrtmp_jobs_total{state=\"failed\"}) or vector(0)",
                "legendFormat": "Failed",
                "refId": "B"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "sum(ffrtmp_jobs_total{state=\"queued\"})",
                "legendFormat": "Queued",
                "refId": "C"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "sum(ffrtmp_jobs_total{state=\"processing\"}) or vector(0)",
                "legendFormat": "Processing",
                "refId": "D"
            }
        ],
        "title": "Jobs by State (Cumulative)",
        "type": "timeseries"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "bars",
                    "fillOpacity": 80,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "linear",
                    "lineWidth": 1,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": False,
                    "stacking": {"group": "A", "mode": "normal"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 4},
        "id": 8,
        "options": {
            "legend": {
                "calcs": ["last"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "none"}
        },
        "targets": [
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_priority{priority=\"high\"} or vector(0)",
                "legendFormat": "High Priority",
                "refId": "A"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_priority{priority=\"medium\"} or vector(0)",
                "legendFormat": "Medium Priority",
                "refId": "B"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_priority{priority=\"low\"} or vector(0)",
                "legendFormat": "Low Priority",
                "refId": "C"
            }
        ],
        "title": "Queue by Priority",
        "type": "timeseries"
    })
    
    # Row 3: Completion Rates and Queue Types
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "Jobs/sec",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "line",
                    "fillOpacity": 20,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "smooth",
                    "lineWidth": 2,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": True,
                    "stacking": {"group": "A", "mode": "none"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 12},
        "id": 9,
        "options": {
            "legend": {
                "calcs": ["mean", "last", "max"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "desc"}
        },
        "targets": [
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "rate(ffrtmp_jobs_total{state=\"completed\"}[5m])",
                "legendFormat": "Completion Rate",
                "refId": "A"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "rate(ffrtmp_jobs_total{state=\"failed\"}[5m]) or vector(0)",
                "legendFormat": "Failure Rate",
                "refId": "B"
            }
        ],
        "title": "Job Completion Rate (per second)",
        "type": "timeseries"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "bars",
                    "fillOpacity": 80,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "linear",
                    "lineWidth": 1,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": False,
                    "stacking": {"group": "A", "mode": "normal"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "short"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 12},
        "id": 10,
        "options": {
            "legend": {
                "calcs": ["last"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "none"}
        },
        "targets": [
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_type{type=\"live\"} or vector(0)",
                "legendFormat": "Live",
                "refId": "A"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_type{type=\"default\"} or vector(0)",
                "legendFormat": "Default",
                "refId": "B"
            },
            {
                "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
                "expr": "ffrtmp_queue_by_type{type=\"batch\"} or vector(0)",
                "legendFormat": "Batch",
                "refId": "C"
            }
        ],
        "title": "Queue by Type",
        "type": "timeseries"
    })
    
    # Row 4: Worker Metrics
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "%",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "line",
                    "fillOpacity": 20,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "smooth",
                    "lineWidth": 2,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": True,
                    "stacking": {"group": "A", "mode": "none"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "max": 100,
                "min": 0,
                "thresholds": {
                    "mode": "absolute",
                    "steps": [
                        {"color": "green", "value": None},
                        {"color": "red", "value": 80}
                    ]
                },
                "unit": "percent"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 20},
        "id": 11,
        "options": {
            "legend": {
                "calcs": ["mean", "last", "max"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "desc"}
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_worker_cpu_usage",
            "legendFormat": "{{node_id}}",
            "refId": "A"
        }],
        "title": "Worker CPU Usage",
        "type": "timeseries"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "axisCenteredZero": False,
                    "axisColorMode": "text",
                    "axisLabel": "",
                    "axisPlacement": "auto",
                    "barAlignment": 0,
                    "drawStyle": "line",
                    "fillOpacity": 20,
                    "gradientMode": "none",
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False},
                    "lineInterpolation": "smooth",
                    "lineWidth": 2,
                    "pointSize": 5,
                    "scaleDistribution": {"type": "linear"},
                    "showPoints": "never",
                    "spanNulls": True,
                    "stacking": {"group": "A", "mode": "none"},
                    "thresholdsStyle": {"mode": "off"}
                },
                "mappings": [],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                },
                "unit": "bytes"
            }
        },
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 20},
        "id": 12,
        "options": {
            "legend": {
                "calcs": ["mean", "last", "max"],
                "displayMode": "table",
                "placement": "bottom",
                "showLegend": True
            },
            "tooltip": {"mode": "multi", "sort": "desc"}
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_worker_memory_bytes",
            "legendFormat": "{{node_id}}",
            "refId": "A"
        }],
        "title": "Worker Memory Usage",
        "type": "timeseries"
    })
    
    # Row 5: Status Distributions
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False}
                },
                "mappings": []
            }
        },
        "gridPos": {"h": 8, "w": 8, "x": 0, "y": 28},
        "id": 13,
        "options": {
            "displayLabels": ["percent"],
            "legend": {
                "displayMode": "table",
                "placement": "right",
                "showLegend": True,
                "values": ["value", "percent"]
            },
            "pieType": "pie",
            "reduceOptions": {
                "calcs": ["lastNotNull"],
                "fields": "",
                "values": False
            },
            "tooltip": {"mode": "single", "sort": "none"}
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_nodes_by_status",
            "legendFormat": "{{status}}",
            "refId": "A"
        }],
        "title": "Worker Status Distribution",
        "type": "piechart"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "palette-classic"},
                "custom": {
                    "hideFrom": {"tooltip": False, "viz": False, "legend": False}
                },
                "mappings": []
            }
        },
        "gridPos": {"h": 8, "w": 8, "x": 8, "y": 28},
        "id": 14,
        "options": {
            "displayLabels": ["percent"],
            "legend": {
                "displayMode": "table",
                "placement": "right",
                "showLegend": True,
                "values": ["value", "percent"]
            },
            "pieType": "donut",
            "reduceOptions": {
                "calcs": ["lastNotNull"],
                "fields": "",
                "values": False
            },
            "tooltip": {"mode": "single", "sort": "none"}
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "ffrtmp_jobs_completed_by_engine",
            "legendFormat": "{{engine}}",
            "refId": "A"
        }],
        "title": "Completed Jobs by Engine",
        "type": "piechart"
    })
    
    panels.append({
        "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
        "fieldConfig": {
            "defaults": {
                "color": {"mode": "thresholds"},
                "custom": {
                    "align": "auto",
                    "cellOptions": {"type": "auto"},
                    "inspect": False
                },
                "mappings": [
                    {
                        "options": {
                            "0": {"color": "red", "index": 0, "text": "Down"},
                            "1": {"color": "green", "index": 1, "text": "Up"}
                        },
                        "type": "value"
                    }
                ],
                "thresholds": {
                    "mode": "absolute",
                    "steps": [{"color": "green", "value": None}]
                }
            }
        },
        "gridPos": {"h": 8, "w": 8, "x": 16, "y": 28},
        "id": 15,
        "options": {
            "cellHeight": "sm",
            "footer": {
                "countRows": False,
                "fields": "",
                "reducer": ["sum"],
                "show": False
            },
            "showHeader": True
        },
        "targets": [{
            "datasource": {"type": "prometheus", "uid": "DS_VICTORIAMETRICS"},
            "expr": "exporter_healthy",
            "format": "table",
            "instant": True,
            "refId": "A"
        }],
        "title": "Exporter Health Status",
        "transformations": [
            {
                "id": "organize",
                "options": {
                    "excludeByName": {
                        "Time": True,
                        "__name__": True,
                        "cluster": True,
                        "environment": True,
                        "instance": True,
                        "job": True
                    },
                    "indexByName": {},
                    "renameByName": {
                        "Value": "Status",
                        "exporter_name": "Exporter"
                    }
                }
            }
        ],
        "type": "table"
    })
    
    dashboard["panels"] = panels
    return dashboard

if __name__ == "__main__":
    dashboard = create_improved_dashboard()
    print(json.dumps(dashboard, indent=2))

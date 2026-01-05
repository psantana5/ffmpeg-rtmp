# Alerting Guide for FFmpeg RTMP

Complete guide for setting up and managing alerts in production FFmpeg RTMP deployments.

## Table of Contents

1. [Overview](#overview)
2. [Alert Categories](#alert-categories)
3. [Setup Instructions](#setup-instructions)
4. [Alert Configuration](#alert-configuration)
5. [Notification Channels](#notification-channels)
6. [Testing Alerts](#testing-alerts)
7. [Incident Response](#incident-response)
8. [Maintenance and Tuning](#maintenance-and-tuning)

---

## Overview

The FFmpeg RTMP system includes comprehensive alerting rules for:
- **Critical incidents** requiring immediate action (page on-call)
- **Warnings** that need investigation (notify team)
- **Performance degradation** for proactive monitoring (info/ticket)

### Alert Severity Levels

| Severity | Response | Notification | Example |
|----------|----------|--------------|---------|
| **critical** | Immediate (page) | PagerDuty + Slack | Master down, all workers offline |
| **warning** | Within 1 hour | Slack + Email | Single worker down, high queue |
| **info** | Next business day | Slack (monitoring) | Performance degradation, low utilization |

---

## Alert Categories

### Critical Alerts (Page Immediately)

**FFmpegMasterNodeDown**
- **Trigger:** Master unreachable for 2+ minutes
- **Impact:** Complete system outage
- **Action:** Check service, logs, restart if needed

**FFmpegAllWorkersDown**
- **Trigger:** Zero workers available for 5+ minutes
- **Impact:** No processing capacity
- **Action:** Check worker services, network connectivity

**FFmpegCriticalFailureRate**
- **Trigger:** >50% jobs failing for 10+ minutes
- **Impact:** System critically degraded
- **Action:** Check worker resources, logs, input files

**FFmpegQueueCritical**
- **Trigger:** Queue > 2000 jobs for 15+ minutes
- **Impact:** Severe delays, SLA violations
- **Action:** Add emergency capacity, check workers

**FFmpegMasterDiskCritical**
- **Trigger:** Master disk < 5% free for 5+ minutes
- **Impact:** Database writes failing
- **Action:** Clean logs, remove backups, expand disk

### Warning Alerts (Investigate)

**FFmpegWorkerNodeDown**
- **Trigger:** Individual worker down 5+ minutes
- **Impact:** Reduced capacity
- **Action:** Check service on worker node

**FFmpegHighFailureRate**
- **Trigger:** >10% jobs failing for 10+ minutes
- **Impact:** Elevated error rate
- **Action:** Investigate failed jobs, check resources

**FFmpegQueueWarning**
- **Trigger:** Queue > 500 jobs for 15+ minutes
- **Impact:** Increasing delays
- **Action:** Monitor growth, plan capacity

**FFmpegWorkerCapacityHigh**
- **Trigger:** Worker > 85% capacity for 30+ minutes
- **Impact:** Limited headroom
- **Action:** Consider adding capacity

**FFmpegHighJobLatency**
- **Trigger:** P95 queue wait > 5 minutes for 15+ minutes
- **Impact:** Slow job processing
- **Action:** Add workers or increase concurrency

**FFmpegWorkerDiskWarning**
- **Trigger:** Worker disk < 20% free for 10+ minutes
- **Impact:** May reject jobs soon
- **Action:** Clean temp files

**FFmpegMasterDiskWarning**
- **Trigger:** Master disk < 15% free for 10+ minutes
- **Impact:** Approaching critical
- **Action:** Clean logs, plan expansion

### Performance Alerts (Informational)

**FFmpegSlowJobExecution**
- **Trigger:** Jobs 50% slower than baseline
- **Impact:** Degraded performance
- **Action:** Investigate CPU, disk, network

**FFmpegLowWorkerUtilization**
- **Trigger:** Worker < 20% utilized for 2+ hours
- **Impact:** Inefficient resource use
- **Action:** Consider cost optimization

---

## Setup Instructions

### Prerequisites

- Prometheus installed and configured
- Alertmanager installed
- Notification channels configured (Slack, PagerDuty, email)

### Step 1: Deploy Alert Rules

**Option A: Docker Compose**
```bash
# Copy alert rules
cp docs/prometheus/ffmpeg-rtmp-alerts.yml deployment/prometheus/rules/

# Mount in docker-compose.yml
volumes:
  - ./deployment/prometheus/rules:/etc/prometheus/rules:ro

# Update prometheus.yml
rule_files:
  - '/etc/prometheus/rules/*.yml'

# Restart Prometheus
docker-compose restart prometheus
```

**Option B: System Installation**
```bash
# Copy rules to Prometheus directory
sudo cp docs/prometheus/ffmpeg-rtmp-alerts.yml /etc/prometheus/rules/

# Fix permissions
sudo chown prometheus:prometheus /etc/prometheus/rules/ffmpeg-rtmp-alerts.yml

# Validate rules
promtool check rules /etc/prometheus/rules/ffmpeg-rtmp-alerts.yml

# Update /etc/prometheus/prometheus.yml
rule_files:
  - '/etc/prometheus/rules/*.yml'

# Reload Prometheus
sudo systemctl reload prometheus
# OR
curl -X POST http://localhost:9090/-/reload
```

### Step 2: Deploy Alertmanager Configuration

**Docker Compose:**
```bash
# Copy config
cp docs/prometheus/alertmanager.yml deployment/prometheus/

# Update docker-compose.yml
alertmanager:
  image: prom/alertmanager:latest
  volumes:
    - ./deployment/prometheus/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
  ports:
    - "9093:9093"

# Start Alertmanager
docker-compose up -d alertmanager
```

**System Installation:**
```bash
# Install Alertmanager
sudo apt-get install prometheus-alertmanager

# Copy config
sudo cp docs/prometheus/alertmanager.yml /etc/alertmanager/

# Set environment variables
echo 'SMTP_PASSWORD=your-password' | sudo tee -a /etc/default/alertmanager
echo 'PAGERDUTY_SERVICE_KEY=your-key' | sudo tee -a /etc/default/alertmanager

# Validate config
amtool check-config /etc/alertmanager/alertmanager.yml

# Start service
sudo systemctl enable alertmanager
sudo systemctl start alertmanager
```

### Step 3: Configure Prometheus to Use Alertmanager

**Add to prometheus.yml:**
```yaml
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - 'localhost:9093'  # or alertmanager:9093 for Docker
```

**Reload Prometheus:**
```bash
# Docker
docker-compose restart prometheus

# System
sudo systemctl reload prometheus
```

---

## Alert Configuration

### Customizing Thresholds

Edit `docs/prometheus/ffmpeg-rtmp-alerts.yml`:

```yaml
# Example: Change queue critical threshold
- alert: FFmpegQueueCritical
  expr: jobs_queued_total{job="ffmpeg-master"} > 3000  # Changed from 2000
  for: 10m  # Changed from 15m
```

### Adding Custom Alerts

```yaml
- name: custom_alerts
  interval: 1m
  rules:
  - alert: CustomAlert
    expr: your_metric > threshold
    for: duration
    labels:
      severity: warning
      component: custom
    annotations:
      summary: "Alert summary"
      description: "Detailed description"
```

### Alert Label Best Practices

Always include these labels:
- `severity`: critical, warning, or info
- `component`: master, worker, queue, database, etc.
- `team`: Team responsible for response

---

## Notification Channels

### Slack Setup

1. **Create Slack App:**
   ```
   https://api.slack.com/apps → Create New App
   ```

2. **Enable Incoming Webhooks:**
   ```
   Features → Incoming Webhooks → Activate
   ```

3. **Add to Workspace:**
   ```
   Add New Webhook to Workspace
   Select channel: #ffmpeg-alerts-critical
   Copy webhook URL
   ```

4. **Configure Alertmanager:**
   ```bash
   # Save webhook URL
   echo "https://hooks.slack.com/services/YOUR/WEBHOOK/URL" | \
     sudo tee /etc/alertmanager/slack_webhook_url
   
   # Or set in alertmanager.yml
   global:
     slack_api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
   ```

**Channels to Create:**
- `#ffmpeg-alerts-critical` - Critical alerts (page team)
- `#ffmpeg-alerts` - Warning alerts
- `#ffmpeg-ops` - Operations team alerts
- `#ffmpeg-monitoring` - Performance/info alerts

### PagerDuty Setup

1. **Create Service:**
   ```
   PagerDuty → Services → New Service
   Name: FFmpeg RTMP Production
   ```

2. **Add Integration:**
   ```
   Integrations → Add Integration
   Type: Prometheus
   Copy Integration Key
   ```

3. **Configure Alertmanager:**
   ```bash
   export PAGERDUTY_SERVICE_KEY="your-integration-key"
   
   # Add to /etc/default/alertmanager or docker-compose.yml
   ```

4. **Set Escalation Policy:**
   ```
   Escalation Policies → Create Policy
   Level 1: On-call engineer (immediate)
   Level 2: Team lead (after 15 min)
   Level 3: Manager (after 30 min)
   ```

### Email Setup

**Gmail Example:**
```yaml
global:
  smtp_from: 'alerts@company.com'
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_auth_username: 'alerts@company.com'
  smtp_auth_password: 'app-specific-password'  # Generate in Gmail settings
  smtp_require_tls: true

receivers:
  - name: 'email-ops'
    email_configs:
      - to: 'ops-team@company.com'
        headers:
          Subject: '[FFmpeg RTMP] {{ .GroupLabels.alertname }}'
```

**SendGrid Example:**
```yaml
global:
  smtp_smarthost: 'smtp.sendgrid.net:587'
  smtp_auth_username: 'apikey'
  smtp_auth_password: '${SENDGRID_API_KEY}'
```

---

## Testing Alerts

### Test Alert Rules

```bash
# Validate syntax
promtool check rules /etc/prometheus/rules/ffmpeg-rtmp-alerts.yml

# Test specific query
promtool query instant http://localhost:9090 \
  'up{job="ffmpeg-master"} == 0'
```

### Test Alertmanager Config

```bash
# Validate config
amtool check-config /etc/alertmanager/alertmanager.yml

# Test routing
amtool config routes test \
  --config.file=/etc/alertmanager/alertmanager.yml \
  --tree \
  severity=critical component=master
```

### Send Test Alert

**Manual test alert:**
```bash
curl -X POST http://localhost:9093/api/v1/alerts -d '[
  {
    "labels": {
      "alertname": "TestAlert",
      "severity": "warning",
      "component": "test"
    },
    "annotations": {
      "summary": "This is a test alert",
      "description": "Testing the alerting pipeline"
    },
    "startsAt": "'$(date -Iseconds)'",
    "endsAt": "'$(date -Iseconds -d '+5 minutes')'"
  }
]'
```

**Trigger real alert (temporary):**
```bash
# Stop master to trigger alert
sudo systemctl stop ffmpeg-master

# Wait 2+ minutes for alert to fire
# Check Prometheus: http://localhost:9090/alerts
# Check Alertmanager: http://localhost:9093

# Restart master
sudo systemctl start ffmpeg-master
```

### Verify Alert Delivery

1. **Check Prometheus Alerts:**
   ```
   http://localhost:9090/alerts
   ```

2. **Check Alertmanager:**
   ```
   http://localhost:9093/#/alerts
   ```

3. **Check Notification Channels:**
   - Slack: Look for test message
   - PagerDuty: Check incidents page
   - Email: Check inbox

---

## Incident Response

### Alert Response Workflow

1. **Receive Alert**
   - Critical: PagerDuty page
   - Warning: Slack notification
   - Info: Email/ticket

2. **Acknowledge**
   - PagerDuty: Acknowledge incident
   - Slack: React with 👀 emoji
   - Update team on status

3. **Investigate**
   - Click dashboard link in alert
   - Check master/worker logs
   - Review recent changes

4. **Remediate**
   - Follow runbook for alert type
   - Apply fix
   - Monitor for resolution

5. **Close**
   - Verify alert resolved in Alertmanager
   - Update incident notes
   - Schedule postmortem if needed

### Runbooks

See **[docs/INCIDENT_PLAYBOOKS.md](INCIDENT_PLAYBOOKS.md)** for detailed response procedures:

- Master Down
- All Workers Down
- High Failure Rate
- Queue Overload
- Disk Space Issues
- Performance Degradation

---

## Maintenance and Tuning

### Regular Maintenance

**Weekly:**
- Review alert statistics
- Check for noisy alerts (tune thresholds)
- Verify notification delivery

**Monthly:**
- Review and update alert thresholds
- Update runbook documentation
- Test disaster recovery procedures

### Silence Alerts During Maintenance

**Web UI:**
```
http://localhost:9093/#/silences
Click "New Silence"
Set matchers: alertname=FFmpegWorkerNodeDown, instance=worker1
Duration: 2 hours
Comment: Scheduled maintenance
```

**CLI:**
```bash
amtool silence add \
  alertname=FFmpegWorkerNodeDown \
  instance=worker1 \
  --duration=2h \
  --comment="Scheduled maintenance"
```

**API:**
```bash
curl -XPOST http://localhost:9093/api/v1/silences -d '{
  "matchers": [
    {"name": "alertname", "value": "FFmpegWorkerNodeDown"},
    {"name": "instance", "value": "worker1"}
  ],
  "startsAt": "'$(date -Iseconds)'",
  "endsAt": "'$(date -Iseconds -d '+2 hours)'",
  "createdBy": "operator@company.com",
  "comment": "Scheduled maintenance"
}'
```

### Alert Tuning

**Reduce false positives:**
- Increase `for` duration
- Add more specific label matchers
- Adjust thresholds based on baseline

**Reduce alert fatigue:**
- Use inhibition rules
- Group related alerts
- Increase repeat_interval

**Example tuning:**
```yaml
# Before: Too sensitive
- alert: FFmpegHighFailureRate
  expr: rate(jobs_failed_total[5m]) > 0.1
  for: 5m

# After: More reasonable
- alert: FFmpegHighFailureRate
  expr: |
    (rate(jobs_failed_total[5m]) / 
     (rate(jobs_completed_total[5m]) + rate(jobs_failed_total[5m]))) > 0.1
  for: 10m  # Increased duration
```

### Metrics for Alert Health

**Alert firing rate:**
```promql
rate(alertmanager_alerts_received_total[1h])
```

**Alert resolution time:**
```promql
histogram_quantile(0.95, 
  rate(alertmanager_notification_latency_seconds_bucket[1h]))
```

**Failed notifications:**
```promql
rate(alertmanager_notifications_failed_total[1h])
```

---

## Troubleshooting

### Alerts Not Firing

1. **Check Prometheus targets:**
   ```
   http://localhost:9090/targets
   Ensure ffmpeg-master and ffmpeg-worker are UP
   ```

2. **Test alert expression:**
   ```
   http://localhost:9090/graph
   Run the expr from alert rule
   Should return data when condition met
   ```

3. **Check alert state:**
   ```
   http://localhost:9090/alerts
   State should be: inactive → pending → firing
   ```

### Alerts Not Notifying

1. **Check Alertmanager received alert:**
   ```
   http://localhost:9093/#/alerts
   ```

2. **Check routing:**
   ```bash
   amtool config routes show
   ```

3. **Check notification logs:**
   ```bash
   # Docker
   docker-compose logs alertmanager | grep -i notification
   
   # System
   journalctl -u alertmanager -f
   ```

4. **Test receiver manually:**
   ```bash
   # Test Slack webhook
   curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK \
     -d '{"text":"Test message"}'
   ```

### Common Issues

**Issue: Slack notifications not arriving**
- Check webhook URL is correct
- Verify Slack app has permissions
- Check Alertmanager logs for errors

**Issue: PagerDuty not creating incidents**
- Verify integration key is correct
- Check PagerDuty service is active
- Ensure escalation policy configured

**Issue: Too many alerts**
- Review inhibition rules
- Increase alert thresholds
- Add grouping/aggregation

---

## Additional Resources

- **[Prometheus Alerting Documentation](https://prometheus.io/docs/alerting/latest/overview/)**
- **[Alertmanager Configuration](https://prometheus.io/docs/alerting/latest/configuration/)**
- **[PromQL Guide](https://prometheus.io/docs/prometheus/latest/querying/basics/)**
- **[Incident Playbooks](INCIDENT_PLAYBOOKS.md)**
- **[Production Operations](PRODUCTION_OPERATIONS.md)**

---

**Version**: 1.0  
**Last Updated**: 2026-01-05  
**Status**: Production Ready

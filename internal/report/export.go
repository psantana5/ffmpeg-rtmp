package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"fmt"
	"strings"
)

// PrometheusExport generates Prometheus metrics in text format
// Layer 2 visibility: boring counters only
func PrometheusExport() string {
	snapshot := Global().Snapshot()
	
	var b strings.Builder
	
	// Help text
	b.WriteString("# HELP ffrtmp_jobs_total Total jobs by state\n")
	b.WriteString("# TYPE ffrtmp_jobs_total counter\n")
	
	// Job lifecycle
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_total{state=\"started\"} %d\n", snapshot["jobs_started"]))
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_total{state=\"completed\"} %d\n", snapshot["jobs_completed"]))
	
	// Platform SLA (most important)
	b.WriteString("\n# HELP ffrtmp_platform_sla_total Platform SLA compliance\n")
	b.WriteString("# TYPE ffrtmp_platform_sla_total counter\n")
	b.WriteString(fmt.Sprintf("ffrtmp_platform_sla_total{compliant=\"true\"} %d\n", snapshot["jobs_platform_compliant"]))
	b.WriteString(fmt.Sprintf("ffrtmp_platform_sla_total{compliant=\"false\"} %d\n", snapshot["jobs_platform_violation"]))
	
	// Mode
	b.WriteString("\n# HELP ffrtmp_jobs_by_mode_total Jobs by execution mode\n")
	b.WriteString("# TYPE ffrtmp_jobs_by_mode_total counter\n")
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_by_mode_total{mode=\"run\"} %d\n", snapshot["jobs_run"]))
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_by_mode_total{mode=\"attach\"} %d\n", snapshot["jobs_attach"]))
	
	// Exit codes
	b.WriteString("\n# HELP ffrtmp_jobs_by_exit_total Jobs by exit code\n")
	b.WriteString("# TYPE ffrtmp_jobs_by_exit_total counter\n")
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_by_exit_total{exit=\"0\"} %d\n", snapshot["jobs_exit_zero"]))
	b.WriteString(fmt.Sprintf("ffrtmp_jobs_by_exit_total{exit=\"non_zero\"} %d\n", snapshot["jobs_exit_non_zero"]))
	
	// Derived metric (SLA rate) - optional but useful
	completed := snapshot["jobs_completed"]
	if completed > 0 {
		compliant := snapshot["jobs_platform_compliant"]
		slaRate := float64(compliant) / float64(completed)
		b.WriteString("\n# HELP ffrtmp_platform_sla_rate Platform SLA compliance rate (0-1)\n")
		b.WriteString("# TYPE ffrtmp_platform_sla_rate gauge\n")
		b.WriteString(fmt.Sprintf("ffrtmp_platform_sla_rate %.6f\n", slaRate))
	}
	
	return b.String()
}

// ViolationsJSON exports recent violations in JSON format
// This is the killer feature: instant root cause without log diving
func ViolationsJSON() string {
	violations := GlobalViolations().GetRecent(50)
	
	if len(violations) == 0 {
		return "[]"
	}
	
	var b strings.Builder
	b.WriteString("[\n")
	
	for i, v := range violations {
		b.WriteString(fmt.Sprintf("  {\"job_id\":\"%s\",\"reason\":\"%s\",\"duration\":%.2f,\"exit_code\":%d,\"pid\":%d}",
			v.JobID, v.Reason, v.Duration, v.ExitCode, v.PID))
		if i < len(violations)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	
	b.WriteString("]")
	return b.String()
}

package prometheus

import (
"testing"
"time"

"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// TestPlatformSLACompliance tests that SLA is based on platform behavior, not job success
func TestPlatformSLACompliance(t *testing.T) {
now := time.Now()

tests := []struct {
name           string
job            *models.Job
expectedSLA    bool
expectedReason string
}{
{
name: "Success - Platform compliant",
job: &models.Job{
ID:             "job-success",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusCompleted,
CreatedAt:      now.Add(-320 * time.Second),
StartedAt:      &[]time.Time{now.Add(-300 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    true,
expectedReason: "compliant",
},
{
name: "User error - Platform compliant (not our fault)",
job: &models.Job{
ID:             "job-user-error",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusFailed,
FailureReason:  models.FailureReasonUserError,
CreatedAt:      now.Add(-120 * time.Second),
StartedAt:      &[]time.Time{now.Add(-100 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    true,
expectedReason: "external_failure_platform_ok",
},
{
name: "Network error - Platform compliant (external issue)",
job: &models.Job{
ID:             "job-network-error",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusFailed,
FailureReason:  models.FailureReasonNetworkError,
CreatedAt:      now.Add(-120 * time.Second),
StartedAt:      &[]time.Time{now.Add(-100 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    true,
expectedReason: "external_failure_platform_ok",
},
{
name: "Platform error - Platform violation (our fault)",
job: &models.Job{
ID:             "job-platform-error",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusFailed,
FailureReason:  models.FailureReasonPlatformError,
CreatedAt:      now.Add(-120 * time.Second),
StartedAt:      &[]time.Time{now.Add(-100 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    false,
expectedReason: "platform_failure",
},
{
name: "Queue time exceeded - Platform violation",
job: &models.Job{
ID:             "job-queue-violation",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusCompleted,
CreatedAt:      now.Add(-60 * time.Second),
StartedAt:      &[]time.Time{now.Add(-20 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    false,
expectedReason: "queue_time_exceeded",
},
{
name: "Processing time exceeded - Platform violation",
job: &models.Job{
ID:             "job-processing-violation",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusCompleted,
CreatedAt:      now.Add(-670 * time.Second),
StartedAt:      &[]time.Time{now.Add(-650 * time.Second)}[0],
CompletedAt:    &now,
},
expectedSLA:    false,
expectedReason: "processing_time_exceeded",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
slaTargets := models.GetDefaultSLATimingTargets()
compliant, reason := tt.job.CalculatePlatformSLACompliance(slaTargets)

if compliant != tt.expectedSLA {
t.Errorf("Expected SLA compliant=%v, got=%v", tt.expectedSLA, compliant)
}

if reason != tt.expectedReason {
t.Errorf("Expected reason=%s, got=%s", tt.expectedReason, reason)
}
})
}
}

// TestPlatformSLAVsJobSuccess demonstrates the key difference
func TestPlatformSLAVsJobSuccess(t *testing.T) {
now := time.Now()

// CRITICAL: Job failed due to bad input, but platform behaved correctly
badInputJob := &models.Job{
ID:             "job-bad-input",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusFailed,
FailureReason:  models.FailureReasonInputError,
CreatedAt:      now.Add(-120 * time.Second),
StartedAt:      &[]time.Time{now.Add(-100 * time.Second)}[0],
CompletedAt:    &now,
}

slaTargets := models.GetDefaultSLATimingTargets()
compliant, reason := badInputJob.CalculatePlatformSLACompliance(slaTargets)

if !compliant {
t.Error("Job with bad input should be platform SLA compliant (not our fault)")
}

if reason != "external_failure_platform_ok" {
t.Errorf("Expected external_failure_platform_ok, got: %s", reason)
}

// Job succeeded BUT platform exceeded queue time
slowQueueJob := &models.Job{
ID:             "job-slow-queue",
Classification: models.JobClassificationProduction,
Status:         models.JobStatusCompleted,
CreatedAt:      now.Add(-60 * time.Second),
StartedAt:      &[]time.Time{now.Add(-20 * time.Second)}[0],
CompletedAt:    &now,
}

compliant2, reason2 := slowQueueJob.CalculatePlatformSLACompliance(slaTargets)

if compliant2 {
t.Error("Job with 40s queue time should violate platform SLA (target: 30s)")
}

if reason2 != "queue_time_exceeded" {
t.Errorf("Expected queue_time_exceeded, got: %s", reason2)
}
}

package agent

import (
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// Engine represents a transcoding engine (FFmpeg, GStreamer, etc.)
type Engine interface {
	// Name returns the engine name
	Name() string
	
	// Supports checks if this engine can handle the given job with worker capabilities
	Supports(job *models.Job, caps *models.NodeCapabilities) bool
	
	// BuildCommand generates the command line arguments for executing the job
	BuildCommand(job *models.Job, hostURL string) ([]string, error)
}

// EngineType represents the type of transcoding engine
type EngineType string

const (
	EngineTypeAuto      EngineType = "auto"
	EngineTypeFFmpeg    EngineType = "ffmpeg"
	EngineTypeGStreamer EngineType = "gstreamer"
)

// JobSpec contains the information needed for engine selection and command building
type JobSpec struct {
	ID         string
	Scenario   string
	Parameters map[string]interface{}
	Queue      string
}

// ToJobSpec converts a models.Job to JobSpec
func ToJobSpec(job *models.Job) JobSpec {
	return JobSpec{
		ID:         job.ID,
		Scenario:   job.Scenario,
		Parameters: job.Parameters,
		Queue:      job.Queue,
	}
}

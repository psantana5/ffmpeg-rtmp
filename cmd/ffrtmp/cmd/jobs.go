package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	// Job submit flags
	scenario   string
	duration   int
	bitrate    string
	confidence string
)

// jobsCmd represents the jobs command
var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage jobs",
	Long:  `Commands for creating, listing, and managing jobs in the ffmpeg-rtmp distributed system.`,
}

// jobsSubmitCmd represents the jobs submit command
var jobsSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a new job",
	Long:  `Submit a new transcoding job to the master server.`,
	RunE:  runJobsSubmit,
}

// jobsStatusCmd represents the jobs status command
var jobsStatusCmd = &cobra.Command{
	Use:   "status <job-id>",
	Short: "Get job status",
	Long:  `Retrieve the status of a specific job by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsStatus,
}

func init() {
	rootCmd.AddCommand(jobsCmd)
	jobsCmd.AddCommand(jobsSubmitCmd)
	jobsCmd.AddCommand(jobsStatusCmd)

	// Flags for job submit
	jobsSubmitCmd.Flags().StringVar(&scenario, "scenario", "", "scenario name (required, e.g., 4K60-h264)")
	jobsSubmitCmd.Flags().IntVar(&duration, "duration", 0, "duration in seconds")
	jobsSubmitCmd.Flags().StringVar(&bitrate, "bitrate", "", "target bitrate (e.g., 10M)")
	jobsSubmitCmd.Flags().StringVar(&confidence, "confidence", "auto", "confidence level (auto, high, medium, low)")
	jobsSubmitCmd.MarkFlagRequired("scenario")
}

type jobRequest struct {
	Scenario   string                 `json:"scenario"`
	Confidence string                 `json:"confidence,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type jobResponse struct {
	ID          string                 `json:"id"`
	Scenario    string                 `json:"scenario"`
	Confidence  string                 `json:"confidence"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Status      string                 `json:"status"`
	NodeID      string                 `json:"node_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	Error       string                 `json:"error,omitempty"`
}

func runJobsSubmit(cmd *cobra.Command, args []string) error {
	url := fmt.Sprintf("%s/jobs", GetMasterURL())

	// Build parameters
	params := make(map[string]interface{})
	if duration > 0 {
		params["duration"] = duration
	}
	if bitrate != "" {
		params["bitrate"] = bitrate
	}

	req := jobRequest{
		Scenario:   scenario,
		Confidence: confidence,
	}
	if len(params) > 0 {
		req.Parameters = params
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to connect to master API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result jobResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if IsJSONOutput() {
		// Output as JSON
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		// Output as table
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Field", "Value")

		table.Append("Job ID", result.ID)
		table.Append("Scenario", result.Scenario)
		table.Append("Confidence", result.Confidence)
		table.Append("Status", result.Status)
		table.Append("Created At", result.CreatedAt.Format(time.RFC3339))

		table.Render()
		fmt.Printf("\nJob submitted successfully! Job ID: %s\n", result.ID)
	}

	return nil
}

func runJobsStatus(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	url := fmt.Sprintf("%s/jobs/%s", GetMasterURL(), jobID)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to master API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result jobResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if IsJSONOutput() {
		// Output as JSON
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		// Output as table
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Field", "Value")

		table.Append("Job ID", result.ID)
		table.Append("Scenario", result.Scenario)
		table.Append("Confidence", result.Confidence)
		table.Append("Status", result.Status)
		table.Append("Retry Count", fmt.Sprintf("%d", result.RetryCount))
		
		if result.NodeID != "" {
			table.Append("Node ID", result.NodeID)
		}

		table.Append("Created At", result.CreatedAt.Format(time.RFC3339))
		
		if result.StartedAt != nil {
			table.Append("Started At", result.StartedAt.Format(time.RFC3339))
		}
		
		if result.CompletedAt != nil {
			table.Append("Completed At", result.CompletedAt.Format(time.RFC3339))
		}

		if result.Error != "" {
			table.Append("Error", result.Error)
		}

		// Display parameters if any
		if len(result.Parameters) > 0 {
			paramsJSON, _ := json.MarshalIndent(result.Parameters, "", "  ")
			table.Append("Parameters", string(paramsJSON))
		}

		table.Render()
	}

	return nil
}

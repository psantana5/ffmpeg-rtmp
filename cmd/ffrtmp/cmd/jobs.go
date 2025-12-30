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
	queue      string
	priority   string
	engine     string
	
	// Job status flags
	followStatus bool
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

// jobsCancelCmd represents the jobs cancel command
var jobsCancelCmd = &cobra.Command{
	Use:   "cancel <job-id>",
	Short: "Cancel a job",
	Long:  `Cancel a pending or running job.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsCancel,
}

// jobsPauseCmd represents the jobs pause command
var jobsPauseCmd = &cobra.Command{
	Use:   "pause <job-id>",
	Short: "Pause a job",
	Long:  `Pause a running job.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsPause,
}

// jobsResumeCmd represents the jobs resume command
var jobsResumeCmd = &cobra.Command{
	Use:   "resume <job-id>",
	Short: "Resume a paused job",
	Long:  `Resume a previously paused job.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsResume,
}

func init() {
	rootCmd.AddCommand(jobsCmd)
	jobsCmd.AddCommand(jobsSubmitCmd)
	jobsCmd.AddCommand(jobsStatusCmd)
	jobsCmd.AddCommand(jobsCancelCmd)
	jobsCmd.AddCommand(jobsPauseCmd)
	jobsCmd.AddCommand(jobsResumeCmd)

	// Flags for job submit
	jobsSubmitCmd.Flags().StringVar(&scenario, "scenario", "", "scenario name (required, e.g., 4K60-h264)")
	jobsSubmitCmd.Flags().IntVar(&duration, "duration", 0, "duration in seconds")
	jobsSubmitCmd.Flags().StringVar(&bitrate, "bitrate", "", "target bitrate (e.g., 10M)")
	jobsSubmitCmd.Flags().StringVar(&confidence, "confidence", "auto", "confidence level (auto, high, medium, low)")
	jobsSubmitCmd.Flags().StringVar(&queue, "queue", "default", "queue type (live, default, batch)")
	jobsSubmitCmd.Flags().StringVar(&priority, "priority", "medium", "priority level (high, medium, low)")
	jobsSubmitCmd.Flags().StringVar(&engine, "engine", "ffmpeg", "encoding engine (ffmpeg, gstreamer)")
	jobsSubmitCmd.MarkFlagRequired("scenario")
	
	// Flags for job status
	jobsStatusCmd.Flags().BoolVar(&followStatus, "follow", false, "poll job status every 2 seconds until completion")
}

type jobRequest struct {
	Scenario   string                 `json:"scenario"`
	Confidence string                 `json:"confidence,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Queue      string                 `json:"queue,omitempty"`
	Priority   string                 `json:"priority,omitempty"`
}

type jobResponse struct {
	ID          string                 `json:"id"`
	Scenario    string                 `json:"scenario"`
	Confidence  string                 `json:"confidence"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Status      string                 `json:"status"`
	Queue       string                 `json:"queue,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	Progress    int                    `json:"progress,omitempty"`
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
	if engine != "" && engine != "ffmpeg" {
		params["engine"] = engine
	}

	req := jobRequest{
		Scenario:   scenario,
		Confidence: confidence,
		Queue:      queue,
		Priority:   priority,
	}
	if len(params) > 0 {
		req.Parameters = params
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create authenticated POST request
	httpReq, err := CreateAuthenticatedRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
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
	
	if followStatus {
		// Follow mode: poll every 2 seconds
		fmt.Printf("Following job %s (press Ctrl+C to stop)...\n\n", jobID)
		for {
			result, err := fetchJobStatus(jobID)
			if err != nil {
				return err
			}
			
			// Clear screen and display status
			fmt.Print("\033[H\033[2J")  // Clear screen
			displayJobStatus(result, false)
			
			// Check if job is in terminal state
			if result.Status == "completed" || result.Status == "failed" || result.Status == "canceled" {
				fmt.Println("\n✓ Job reached terminal state")
				break
			}
			
			time.Sleep(2 * time.Second)
		}
	} else {
		// Single fetch mode
		result, err := fetchJobStatus(jobID)
		if err != nil {
			return err
		}
		displayJobStatus(result, true)
	}
	
	return nil
}

func fetchJobStatus(jobID string) (*jobResponse, error) {
	url := fmt.Sprintf("%s/jobs/%s", GetMasterURL(), jobID)

	// Create authenticated GET request
	httpReq, err := CreateAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to master API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result jobResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &result, nil
}

func displayJobStatus(result *jobResponse, renderTable bool) {
	if IsJSONOutput() {
		// Output as JSON
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return
	}
	
	// Output as table
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Field", "Value")

	table.Append([]string{"Job ID", result.ID})
	table.Append([]string{"Scenario", result.Scenario})
	table.Append([]string{"Confidence", result.Confidence})
	table.Append([]string{"Status", result.Status})
	
	if result.Queue != "" {
		table.Append([]string{"Queue", result.Queue})
	}
	if result.Priority != "" {
		table.Append([]string{"Priority", result.Priority})
	}
	if result.Progress > 0 {
		table.Append([]string{"Progress", fmt.Sprintf("%d%%", result.Progress)})
	}
	
	table.Append([]string{"Retry Count", fmt.Sprintf("%d", result.RetryCount)})
	
	if result.NodeID != "" {
		table.Append([]string{"Node ID", result.NodeID})
	}

	table.Append([]string{"Created At", result.CreatedAt.Format(time.RFC3339)})
	
	if result.StartedAt != nil {
		table.Append([]string{"Started At", result.StartedAt.Format(time.RFC3339)})
	}
	
	if result.CompletedAt != nil {
		table.Append([]string{"Completed At", result.CompletedAt.Format(time.RFC3339)})
	}

	if result.Error != "" {
		table.Append([]string{"Error", result.Error})
	}

	// Display parameters if any
	if len(result.Parameters) > 0 {
		paramsJSON, _ := json.MarshalIndent(result.Parameters, "", "  ")
		table.Append([]string{"Parameters", string(paramsJSON)})
	}

	if renderTable {
		table.Render()
	} else {
		table.Render()
	}
}

func runJobsCancel(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	return controlJob(jobID, "cancel")
}

func runJobsPause(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	return controlJob(jobID, "pause")
}

func runJobsResume(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	return controlJob(jobID, "resume")
}

func controlJob(jobID, action string) error {
	url := fmt.Sprintf("%s/jobs/%s/%s", GetMasterURL(), jobID, action)

	// Create authenticated POST request
	httpReq, err := CreateAuthenticatedRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
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

	fmt.Printf("✓ Job %s %sed successfully\n", jobID, action)
	return nil
}

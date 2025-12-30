package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// nodesCmd represents the nodes command
var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Manage compute nodes",
	Long:  `Commands for listing and managing compute nodes in the ffmpeg-rtmp distributed system.`,
}

// nodesListCmd represents the nodes list command
var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered nodes",
	Long:  `Retrieve and display all registered compute nodes from the master server.`,
	RunE:  runNodesList,
}

func init() {
	rootCmd.AddCommand(nodesCmd)
	nodesCmd.AddCommand(nodesListCmd)
}

type nodesListResponse struct {
	Nodes []nodeInfo `json:"nodes"`
	Count int        `json:"count"`
}

type nodeInfo struct {
	ID            string `json:"id"`
	Address       string `json:"address"`
	Type          string `json:"type"`
	CPUThreads    int    `json:"cpu_threads"`
	CPUModel      string `json:"cpu_model"`
	HasGPU        bool   `json:"has_gpu"`
	GPUType       string `json:"gpu_type,omitempty"`
	Status        string `json:"status"`
	CurrentJobID  string `json:"current_job_id,omitempty"`
}

func runNodesList(cmd *cobra.Command, args []string) error {
	url := fmt.Sprintf("%s/nodes", GetMasterURL())

	// Create authenticated GET request
	httpReq, err := CreateAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect to master API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result nodesListResponse
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
		if len(result.Nodes) == 0 {
			fmt.Println("No nodes registered")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header("ID", "Host", "Status", "Type", "CPU", "GPU")

		for _, node := range result.Nodes {
			gpuInfo := "No"
			if node.HasGPU {
				gpuInfo = "Yes"
				if node.GPUType != "" {
					gpuInfo = node.GPUType
				}
			}

			cpuInfo := fmt.Sprintf("%d threads", node.CPUThreads)
			if node.CPUModel != "" {
				cpuInfo = fmt.Sprintf("%s (%d)", node.CPUModel, node.CPUThreads)
			}

			table.Append(
				node.ID,
				node.Address,
				node.Status,
				node.Type,
				cpuInfo,
				gpuInfo,
			)
		}

		table.Render()
		fmt.Printf("\nTotal nodes: %d\n", result.Count)
	}

	return nil
}

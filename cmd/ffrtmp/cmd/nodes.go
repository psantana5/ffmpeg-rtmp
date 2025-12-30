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

// nodesDescribeCmd represents the nodes describe command
var nodesDescribeCmd = &cobra.Command{
	Use:   "describe <node-id>",
	Short: "Get detailed information about a node",
	Long:  `Retrieve detailed hardware capabilities, load, and active jobs for a specific node.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNodesDescribe,
}

func init() {
	rootCmd.AddCommand(nodesCmd)
	nodesCmd.AddCommand(nodesListCmd)
	nodesCmd.AddCommand(nodesDescribeCmd)
}

type nodesListResponse struct {
	Nodes []nodeInfo `json:"nodes"`
	Count int        `json:"count"`
}

type nodeInfo struct {
	ID              string   `json:"id"`
	Address         string   `json:"address"`
	Type            string   `json:"type"`
	CPUThreads      int      `json:"cpu_threads"`
	CPUModel        string   `json:"cpu_model"`
	CPULoadPercent  float64  `json:"cpu_load_percent,omitempty"`
	HasGPU          bool     `json:"has_gpu"`
	GPUType         string   `json:"gpu_type,omitempty"`
	GPUCapabilities []string `json:"gpu_capabilities,omitempty"`
	RAMTotalBytes   uint64   `json:"ram_total_bytes,omitempty"`
	RAMFreeBytes    uint64   `json:"ram_free_bytes,omitempty"`
	Status          string   `json:"status"`
	CurrentJobID    string   `json:"current_job_id,omitempty"`
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

func runNodesDescribe(cmd *cobra.Command, args []string) error {
	nodeID := args[0]
	url := fmt.Sprintf("%s/nodes/%s", GetMasterURL(), nodeID)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var node nodeInfo
	if err := json.Unmarshal(body, &node); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if IsJSONOutput() {
		// Output as JSON
		output, err := json.MarshalIndent(node, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		// Output as detailed table
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Property", "Value")

		table.Append([]string{"Node ID", node.ID})
		table.Append([]string{"Address", node.Address})
		table.Append([]string{"Type", node.Type})
		table.Append([]string{"Status", node.Status})

		// CPU Information
		cpuInfo := fmt.Sprintf("%d threads", node.CPUThreads)
		if node.CPUModel != "" {
			cpuInfo = fmt.Sprintf("%s (%d threads)", node.CPUModel, node.CPUThreads)
		}
		table.Append([]string{"CPU", cpuInfo})
		
		if node.CPULoadPercent > 0 {
			table.Append([]string{"CPU Load", fmt.Sprintf("%.1f%%", node.CPULoadPercent)})
		}

		// GPU Information
		if node.HasGPU {
			gpuInfo := "Yes"
			if node.GPUType != "" {
				gpuInfo = node.GPUType
			}
			table.Append([]string{"GPU", gpuInfo})
			
			if len(node.GPUCapabilities) > 0 {
				for i, cap := range node.GPUCapabilities {
					label := "GPU Capabilities"
					if i > 0 {
						label = ""
					}
					table.Append([]string{label, cap})
				}
			}
		} else {
			table.Append([]string{"GPU", "No"})
		}

		// Memory Information
		if node.RAMTotalBytes > 0 {
			totalGB := float64(node.RAMTotalBytes) / (1024 * 1024 * 1024)
			table.Append([]string{"Total RAM", fmt.Sprintf("%.2f GB", totalGB)})
		}
		if node.RAMFreeBytes > 0 {
			freeGB := float64(node.RAMFreeBytes) / (1024 * 1024 * 1024)
			table.Append([]string{"Free RAM", fmt.Sprintf("%.2f GB", freeGB)})
		}

		// Active Job
		if node.CurrentJobID != "" {
			table.Append([]string{"Active Job", node.CurrentJobID})
		} else {
			table.Append([]string{"Active Job", "None"})
		}

		table.Render()
	}

	return nil
}

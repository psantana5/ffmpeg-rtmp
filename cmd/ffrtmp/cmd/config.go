package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configEnvironment string
	configOutput      string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management and recommendations",
	Long:  `Commands for generating and managing worker configuration based on hardware capabilities.`,
}

var configRecommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Generate recommended worker configuration",
	Long: `Analyzes system hardware (CPU, RAM, GPU) and generates optimal worker
configuration parameters. Takes into account the deployment environment
(development, staging, production) to provide safe, performant defaults.`,
	RunE: runConfigRecommend,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configRecommendCmd)

	configRecommendCmd.Flags().StringVarP(&configEnvironment, "environment", "e", "development",
		"Deployment environment: development, staging, production")
	configRecommendCmd.Flags().StringVarP(&configOutput, "output", "o", "text",
		"Output format: text, json, yaml, bash")
}

type ConfigRecommendation struct {
	Hardware      HardwareInfo      `json:"hardware" yaml:"hardware"`
	Recommendations WorkerConfig     `json:"recommendations" yaml:"recommendations"`
	Rationale      string            `json:"rationale" yaml:"rationale"`
}

type HardwareInfo struct {
	CPUModel     string `json:"cpu_model" yaml:"cpu_model"`
	CPUThreads   int    `json:"cpu_threads" yaml:"cpu_threads"`
	RAMBytes     uint64 `json:"ram_bytes" yaml:"ram_bytes"`
	RAMGB        string `json:"ram_gb" yaml:"ram_gb"`
	HasGPU       bool   `json:"has_gpu" yaml:"has_gpu"`
	GPUType      string `json:"gpu_type,omitempty" yaml:"gpu_type,omitempty"`
	NodeType     string `json:"node_type" yaml:"node_type"`
	OS           string `json:"os" yaml:"os"`
	Architecture string `json:"architecture" yaml:"architecture"`
}

type WorkerConfig struct {
	MaxConcurrentJobs int    `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`
	PollInterval      string `json:"poll_interval" yaml:"poll_interval"`
	MetricsPort       int    `json:"metrics_port" yaml:"metrics_port"`
	WorkDir           string `json:"work_dir" yaml:"work_dir"`
}

func runConfigRecommend(cmd *cobra.Command, args []string) error {
	// Detect hardware
	caps, err := agent.DetectHardware()
	if err != nil {
		return fmt.Errorf("failed to detect hardware: %w", err)
	}

	nodeType := agent.DetectNodeType(caps.CPUThreads, caps.RAMTotalBytes)

	hardware := HardwareInfo{
		CPUModel:     caps.CPUModel,
		CPUThreads:   caps.CPUThreads,
		RAMBytes:     caps.RAMTotalBytes,
		RAMGB:        agent.FormatRAM(caps.RAMTotalBytes),
		HasGPU:       caps.HasGPU,
		GPUType:      caps.GPUType,
		NodeType:     string(nodeType),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// Calculate recommendations
	config := calculateRecommendations(hardware, configEnvironment)
	rationale := generateRationale(hardware, config, configEnvironment)

	recommendation := ConfigRecommendation{
		Hardware:      hardware,
		Recommendations: config,
		Rationale:      rationale,
	}

	// Output in requested format
	return outputRecommendation(recommendation, configOutput)
}

func calculateRecommendations(hw HardwareInfo, environment string) WorkerConfig {
	cores := hw.CPUThreads
	
	// Base calculation: GPU available = 75% of cores, CPU-only = 25% of cores
	var baseConcurrent int
	if hw.HasGPU {
		baseConcurrent = int(float64(cores) * 0.75)
	} else {
		baseConcurrent = int(float64(cores) * 0.25)
	}

	// Environment adjustment
	concurrent := baseConcurrent
	if environment == "development" {
		concurrent = baseConcurrent / 2
		if concurrent < 1 {
			concurrent = 1
		}
	}

	// Apply safety limits based on node type
	maxLimit := getMaxConcurrentLimit(hw.NodeType)
	if concurrent > maxLimit {
		concurrent = maxLimit
	}
	if concurrent < 1 {
		concurrent = 1
	}

	// Poll interval: faster for production
	pollInterval := "5s"
	if environment == "production" {
		pollInterval = "3s"
	}

	return WorkerConfig{
		MaxConcurrentJobs: concurrent,
		PollInterval:      pollInterval,
		MetricsPort:       9091,
		WorkDir:           agent.GetRecommendedWorkDir(),
	}
}

func getMaxConcurrentLimit(nodeType string) int {
	switch models.NodeType(nodeType) {
	case models.NodeTypeLaptop:
		return 4
	case models.NodeTypeDesktop:
		return 6
	case models.NodeTypeServer:
		return 12
	default:
		return 24 // HPC or unknown
	}
}

func generateRationale(hw HardwareInfo, config WorkerConfig, env string) string {
	gpuText := "no GPU"
	if hw.HasGPU {
		gpuText = fmt.Sprintf("GPU (%s)", hw.GPUType)
	}

	envFactor := "base"
	if env == "development" {
		envFactor = "50% (development environment)"
	} else if env == "production" {
		envFactor = "100% (production environment)"
	}

	return fmt.Sprintf(
		"Based on %d CPU threads, %s, and %s: recommended %d concurrent jobs "+
			"(capacity factor: %s, node type limit: %d)",
		hw.CPUThreads,
		hw.RAMGB,
		gpuText,
		config.MaxConcurrentJobs,
		envFactor,
		getMaxConcurrentLimit(hw.NodeType),
	)
}

func outputRecommendation(rec ConfigRecommendation, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(rec)

	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(rec)

	case "bash":
		fmt.Println("# Worker configuration recommendations")
		fmt.Printf("export MAX_CONCURRENT_JOBS=%d\n", rec.Recommendations.MaxConcurrentJobs)
		fmt.Printf("export POLL_INTERVAL=%s\n", rec.Recommendations.PollInterval)
		fmt.Printf("export METRICS_PORT=%d\n", rec.Recommendations.MetricsPort)
		fmt.Printf("export WORK_DIR=%s\n", rec.Recommendations.WorkDir)
		fmt.Println()
		fmt.Printf("# %s\n", rec.Rationale)
		return nil

	default: // text
		fmt.Println("Hardware Configuration:")
		fmt.Printf("  CPU: %s (%d threads)\n", rec.Hardware.CPUModel, rec.Hardware.CPUThreads)
		fmt.Printf("  RAM: %s\n", rec.Hardware.RAMGB)
		fmt.Printf("  GPU: %s", boolToYesNo(rec.Hardware.HasGPU))
		if rec.Hardware.HasGPU {
			fmt.Printf(" (%s)", rec.Hardware.GPUType)
		}
		fmt.Println()
		fmt.Printf("  Node Type: %s\n", rec.Hardware.NodeType)
		fmt.Printf("  OS: %s/%s\n", rec.Hardware.OS, rec.Hardware.Architecture)
		fmt.Println()

		fmt.Println("Recommended Worker Configuration:")
		fmt.Printf("  --max-concurrent-jobs %d\n", rec.Recommendations.MaxConcurrentJobs)
		fmt.Printf("  --poll-interval %s\n", rec.Recommendations.PollInterval)
		fmt.Printf("  --metrics-port %d\n", rec.Recommendations.MetricsPort)
		fmt.Printf("  --work-dir %s\n", rec.Recommendations.WorkDir)
		fmt.Println()

		fmt.Println("Rationale:")
		fmt.Printf("  %s\n", rec.Rationale)
		fmt.Println()

		fmt.Println("Example command:")
		fmt.Printf("  ./bin/agent --master https://MASTER_IP:8080 \\\n")
		fmt.Printf("    --register \\\n")
		fmt.Printf("    --max-concurrent-jobs %d \\\n", rec.Recommendations.MaxConcurrentJobs)
		fmt.Printf("    --poll-interval %s \\\n", rec.Recommendations.PollInterval)
		fmt.Printf("    --metrics-port %d\n", rec.Recommendations.MetricsPort)

		return nil
	}
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

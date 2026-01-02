package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// tenantsCmd represents the tenants command
var tenantsCmd = &cobra.Command{
	Use:   "tenants",
	Short: "Manage tenants in the system",
	Long:  `Create, list, and manage tenants in the multi-tenant ffmpeg-rtmp system.`,
}

// createTenantCmd creates a new tenant
var createTenantCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new tenant",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		displayName, _ := cmd.Flags().GetString("display-name")
		plan, _ := cmd.Flags().GetString("plan")

		reqBody := map[string]interface{}{
			"name": name,
			"plan": plan,
		}
		if displayName != "" {
			reqBody["display_name"] = displayName
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		req, err := http.NewRequest("POST", masterURL+"/tenants", bytes.NewReader(bodyBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		var tenant map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
			os.Exit(1)
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(tenant, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("✓ Tenant created successfully\n")
			fmt.Printf("  ID: %s\n", tenant["id"])
			fmt.Printf("  Name: %s\n", tenant["name"])
			fmt.Printf("  Plan: %s\n", tenant["plan"])
		}
	},
}

// listTenantsCmd lists all tenants
var listTenantsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tenants",
	Run: func(cmd *cobra.Command, args []string) {
		activeOnly, _ := cmd.Flags().GetBool("active")

		url := masterURL + "/tenants"
		if activeOnly {
			url += "?active=true"
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		var tenants []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
			os.Exit(1)
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(tenants, "", "  ")
			fmt.Println(string(output))
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tNAME\tPLAN\tSTATUS\tJOBS\tWORKERS\tCREATED\n")
			for _, t := range tenants {
				id := t["id"].(string)
				name := t["name"].(string)
				plan := t["plan"].(string)
				status := t["status"].(string)
				
				// Parse created_at
				createdAt := ""
				if ca, ok := t["created_at"].(string); ok {
					if ct, err := time.Parse(time.RFC3339, ca); err == nil {
						createdAt = ct.Format("2006-01-02 15:04")
					}
				}

				// Get usage stats if available
				jobs := "-"
				workers := "-"
				if usage, ok := t["usage"].(map[string]interface{}); ok {
					if activeJobs, ok := usage["active_jobs"].(float64); ok {
						jobs = fmt.Sprintf("%.0f", activeJobs)
					}
					if activeWorkers, ok := usage["active_workers"].(float64); ok {
						workers = fmt.Sprintf("%.0f", activeWorkers)
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					id[:8], name, plan, status, jobs, workers, createdAt)
			}
			w.Flush()
		}
	},
}

// getTenantCmd gets details of a specific tenant
var getTenantCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get tenant details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		req, err := http.NewRequest("GET", masterURL+"/tenants/"+tenantID, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		var tenant map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
			os.Exit(1)
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(tenant, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("Tenant Details:\n")
			fmt.Printf("  ID: %s\n", tenant["id"])
			fmt.Printf("  Name: %s\n", tenant["name"])
			if displayName, ok := tenant["display_name"].(string); ok && displayName != "" {
				fmt.Printf("  Display Name: %s\n", displayName)
			}
			fmt.Printf("  Plan: %s\n", tenant["plan"])
			fmt.Printf("  Status: %s\n", tenant["status"])
			
			if quotas, ok := tenant["quotas"].(map[string]interface{}); ok {
				fmt.Printf("\nQuotas:\n")
				if maxJobs, ok := quotas["max_jobs"].(float64); ok {
					fmt.Printf("  Max Jobs: %.0f\n", maxJobs)
				}
				if maxWorkers, ok := quotas["max_workers"].(float64); ok {
					fmt.Printf("  Max Workers: %.0f\n", maxWorkers)
				}
				if maxCPU, ok := quotas["max_cpu_cores"].(float64); ok {
					fmt.Printf("  Max CPU Cores: %.0f\n", maxCPU)
				}
				if maxGPU, ok := quotas["max_gpus"].(float64); ok {
					fmt.Printf("  Max GPUs: %.0f\n", maxGPU)
				}
			}
		}
	},
}

// tenantStatsCmd gets usage statistics for a tenant
var tenantStatsCmd = &cobra.Command{
	Use:   "stats [id]",
	Short: "Get tenant usage statistics",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		req, err := http.NewRequest("GET", masterURL+"/tenants/"+tenantID+"/stats", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
			os.Exit(1)
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("Tenant: %s (%s)\n", stats["name"], stats["plan"])
			fmt.Printf("Status: %s\n\n", stats["status"])

			if usage, ok := stats["usage"].(map[string]interface{}); ok {
				fmt.Printf("Current Usage:\n")
				if v, ok := usage["active_jobs"].(float64); ok {
					fmt.Printf("  Active Jobs: %.0f\n", v)
				}
				if v, ok := usage["active_workers"].(float64); ok {
					fmt.Printf("  Active Workers: %.0f\n", v)
				}
				if v, ok := usage["cpu_cores_used"].(float64); ok {
					fmt.Printf("  CPU Cores Used: %.0f\n", v)
				}
				if v, ok := usage["gpus_used"].(float64); ok {
					fmt.Printf("  GPUs Used: %.0f\n", v)
				}
				if v, ok := usage["jobs_this_hour"].(float64); ok {
					fmt.Printf("  Jobs This Hour: %.0f\n", v)
				}
			}

			if limits, ok := stats["limits"].(map[string]interface{}); ok {
				fmt.Printf("\nAvailable Resources:\n")
				if v, ok := limits["jobs_available"].(float64); ok {
					fmt.Printf("  Jobs Available: %.0f\n", v)
				}
				if v, ok := limits["workers_available"].(float64); ok {
					fmt.Printf("  Workers Available: %.0f\n", v)
				}
				if v, ok := limits["cpu_cores_available"].(float64); ok {
					fmt.Printf("  CPU Cores Available: %.0f\n", v)
				}
				if v, ok := limits["gpus_available"].(float64); ok {
					fmt.Printf("  GPUs Available: %.0f\n", v)
				}
			}
		}
	},
}

// updateTenantCmd updates a tenant
var updateTenantCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update a tenant",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		plan, _ := cmd.Flags().GetString("plan")
		status, _ := cmd.Flags().GetString("status")
		displayName, _ := cmd.Flags().GetString("display-name")

		reqBody := make(map[string]interface{})
		if plan != "" {
			reqBody["plan"] = plan
		}
		if status != "" {
			reqBody["status"] = status
		}
		if displayName != "" {
			reqBody["display_name"] = displayName
		}

		if len(reqBody) == 0 {
			fmt.Fprintf(os.Stderr, "Error: At least one field must be specified for update\n")
			os.Exit(1)
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		req, err := http.NewRequest("PUT", masterURL+"/tenants/"+tenantID, bytes.NewReader(bodyBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		fmt.Println("✓ Tenant updated successfully")
	},
}

// deleteTenantCmd deletes a tenant
var deleteTenantCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a tenant (soft delete)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		req, err := http.NewRequest("DELETE", masterURL+"/tenants/"+tenantID, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
			os.Exit(1)
		}

		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		fmt.Println("✓ Tenant deleted successfully")
	},
}

func init() {
	rootCmd.AddCommand(tenantsCmd)

	// Add subcommands
	tenantsCmd.AddCommand(createTenantCmd)
	tenantsCmd.AddCommand(listTenantsCmd)
	tenantsCmd.AddCommand(getTenantCmd)
	tenantsCmd.AddCommand(updateTenantCmd)
	tenantsCmd.AddCommand(deleteTenantCmd)
	tenantsCmd.AddCommand(tenantStatsCmd)

	// Flags for create
	createTenantCmd.Flags().String("display-name", "", "Display name for the tenant")
	createTenantCmd.Flags().String("plan", "free", "Tenant plan (free, basic, pro, enterprise)")

	// Flags for list
	listTenantsCmd.Flags().Bool("active", false, "Show only active tenants")

	// Flags for update
	updateTenantCmd.Flags().String("plan", "", "Update tenant plan")
	updateTenantCmd.Flags().String("status", "", "Update tenant status (active, suspended, expired)")
	updateTenantCmd.Flags().String("display-name", "", "Update display name")
}

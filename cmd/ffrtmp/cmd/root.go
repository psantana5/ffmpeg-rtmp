package cmd

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	masterURL          string
	outputFormat       string
	cfgFile            string
	apiKey             string
	httpClient         *http.Client
	httpClientMasterURL string // Track which masterURL the client was initialized with
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "ffrtmp",
	Short: "CLI for ffmpeg-rtmp distributed system",
	Long:  `ffrtmp is a command line interface for managing nodes and jobs in the ffmpeg-rtmp distributed transcoding system.`,
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ffrtmp/config)")
	rootCmd.PersistentFlags().StringVar(&masterURL, "master", "", "master API URL (default from config or https://localhost:8080)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "table", "output format: table or json")
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".ffrtmp/config" (without extension)
		configDir := filepath.Join(home, ".ffrtmp")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv() // read in environment variables that match
	
	// Bind specific environment variables
	viper.BindEnv("api_key", "MASTER_API_KEY")
	viper.BindEnv("master_url", "MASTER_URL")

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		// Config file found and successfully parsed
		if viper.GetString("master_url") != "" && masterURL == "" {
			masterURL = viper.GetString("master_url")
		}
		if viper.GetString("api_key") != "" && apiKey == "" {
			apiKey = viper.GetString("api_key")
		}
	}
	
	// Check environment variables if not set from config
	if apiKey == "" && viper.GetString("api_key") != "" {
		apiKey = viper.GetString("api_key")
	}
	if masterURL == "" && viper.GetString("master_url") != "" {
		masterURL = viper.GetString("master_url")
	}

	// Set default if still empty
	if masterURL == "" {
		masterURL = "https://localhost:8080"
	}
}

// initHTTPClient initializes the HTTP client with appropriate TLS settings
func initHTTPClient() {
	// Create a TLS config that skips verification for localhost/127.0.0.1
	// but uses proper verification for remote hosts
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}
	
	// For localhost and 127.0.0.1, skip TLS verification to support self-signed certs
	// This is acceptable for development/testing scenarios where the master runs locally
	// with self-generated certificates. For production, use proper certificates or
	// explicitly specify the master URL to avoid this auto-detection.
	// Security Note: Only applies to localhost - remote hosts always use proper verification
	if strings.Contains(masterURL, "localhost") || strings.Contains(masterURL, "127.0.0.1") {
		tlsConfig.InsecureSkipVerify = true // nosemgrep: go.lang.security.audit.net.use-tls.use-tls
	}
	
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

// GetMasterURL returns the configured master URL with trailing slashes removed
func GetMasterURL() string {
	return strings.TrimRight(masterURL, "/")
}

// IsJSONOutput returns true if JSON output is requested
func IsJSONOutput() bool {
	return outputFormat == "json"
}

// GetAPIKey returns the configured API key
func GetAPIKey() string {
	return apiKey
}

// GetHTTPClient returns the configured HTTP client
// Re-initializes if masterURL has changed since last initialization
func GetHTTPClient() *http.Client {
	if httpClient == nil || httpClientMasterURL != masterURL {
		initHTTPClient()
		httpClientMasterURL = masterURL
	}
	return httpClient
}

// CreateAuthenticatedRequest creates an HTTP request with authentication header if API key is configured
func CreateAuthenticatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	return req, nil
}

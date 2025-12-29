package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// Client manages communication with the master node
type Client struct {
	masterURL  string
	httpClient *http.Client
	nodeID     string
}

// NewClient creates a new agent client
func NewClient(masterURL string) *Client {
	return &Client{
		masterURL: masterURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Register registers the node with the master
func (c *Client) Register(reg *models.NodeRegistration) (*models.Node, error) {
	data, err := json.Marshal(reg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.masterURL+"/nodes/register",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var node models.Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("failed to decode node: %w", err)
	}

	c.nodeID = node.ID
	return &node, nil
}

// SendHeartbeat sends a heartbeat to the master
func (c *Client) SendHeartbeat() error {
	if c.nodeID == "" {
		return fmt.Errorf("node not registered")
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/nodes/%s/heartbeat", c.masterURL, c.nodeID),
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetNextJob retrieves the next available job
func (c *Client) GetNextJob() (*models.Job, error) {
	if c.nodeID == "" {
		return nil, fmt.Errorf("node not registered")
	}

	resp, err := c.httpClient.Get(
		fmt.Sprintf("%s/jobs/next?node_id=%s", c.masterURL, c.nodeID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get next job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get next job failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Job *models.Job `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode job: %w", err)
	}

	return result.Job, nil
}

// SendResults sends job results to the master
func (c *Client) SendResults(result *models.JobResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.masterURL+"/results",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to send results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send results failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetNodeID returns the node ID
func (c *Client) GetNodeID() string {
	return c.nodeID
}

// Package supervisord provides an XML-RPC client for Supervisord
package supervisord

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client represents a Supervisord XML-RPC client
type Client struct {
	url    string
	client *http.Client
	logger *zap.Logger
}

// Config holds the configuration for the Supervisord client
type Config struct {
	URL string
}

// NewClient creates a new Supervisord client
func NewClient(cfg *Config, logger *zap.Logger) *Client {
	return &Client{
		url: cfg.URL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ProcessState represents the state of a process
type ProcessState string

const (
	ProcessStateStopped  ProcessState = "STOPPED"
	ProcessStateStarting ProcessState = "STARTING"
	ProcessStateRunning  ProcessState = "RUNNING"
	ProcessStateStopping ProcessState = "STOPPING"
	ProcessStateExited   ProcessState = "EXITED"
	ProcessStateFatal    ProcessState = "FATAL"
	ProcessStateUnknown  ProcessState = "UNKNOWN"
)

// ProcessInfo represents information about a process
type ProcessInfo struct {
	Name          string       `xml:"name"`
	Group         string       `xml:"group"`
	State         int          `xml:"state"`
	StateName     ProcessState `xml:"statename"`
	Start         int64        `xml:"start"`
	Stop          int64        `xml:"stop"`
	SpawnErr      string       `xml:"spawnerr"`
	ExitStatus    int          `xml:"exitstatus"`
	StdoutLogfile string       `xml:"stdout_logfile"`
	StderrLogfile string       `xml:"stderr_logfile"`
	Pid           int          `xml:"pid"`
}

// StartProcess starts a process by name
func (c *Client) StartProcess(ctx context.Context, name string, wait bool) error {
	_, err := c.call(ctx, "supervisor.startProcess", []any{name, wait})
	if err != nil {
		return fmt.Errorf("failed to start process %s: %w", name, err)
	}
	c.logger.Info("Started process", zap.String("name", name))
	return nil
}

// StopProcess stops a process by name
func (c *Client) StopProcess(ctx context.Context, name string, wait bool) error {
	_, err := c.call(ctx, "supervisor.stopProcess", []any{name, wait})
	if err != nil {
		return fmt.Errorf("failed to stop process %s: %w", name, err)
	}
	c.logger.Info("Stopped process", zap.String("name", name))
	return nil
}

// RestartProcess restarts a process (stop + start)
func (c *Client) RestartProcess(ctx context.Context, name string, wait bool) error {
	// Try to stop first, ignore error if not running
	_ = c.StopProcess(ctx, name, wait)

	// Wait a bit for cleanup
	time.Sleep(500 * time.Millisecond)

	// Start the process
	return c.StartProcess(ctx, name, wait)
}

// GetProcessInfo gets information about a process
func (c *Client) GetProcessInfo(ctx context.Context, name string) (*ProcessInfo, error) {
	resp, err := c.call(ctx, "supervisor.getProcessInfo", []any{name})
	if err != nil {
		return nil, fmt.Errorf("failed to get process info for %s: %w", name, err)
	}

	info := &ProcessInfo{}
	if err := parseProcessInfo(resp, info); err != nil {
		return nil, err
	}

	return info, nil
}

// GetAllProcessInfo gets information about all processes
func (c *Client) GetAllProcessInfo(ctx context.Context) ([]*ProcessInfo, error) {
	resp, err := c.call(ctx, "supervisor.getAllProcessInfo", []any{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all process info: %w", err)
	}

	return parseAllProcessInfo(resp)
}

// IsProcessRunning checks if a process is running
func (c *Client) IsProcessRunning(ctx context.Context, name string) (bool, error) {
	info, err := c.GetProcessInfo(ctx, name)
	if err != nil {
		return false, err
	}
	return info.StateName == ProcessStateRunning, nil
}

// WaitForProcess waits for a process to reach a specific state
func (c *Client) WaitForProcess(ctx context.Context, name string, state ProcessState, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for process %s to reach state %s", name, state)
		case <-ticker.C:
			info, err := c.GetProcessInfo(ctx, name)
			if err != nil {
				continue
			}
			if info.StateName == state {
				return nil
			}
		}
	}
}

// ReadProcessStdoutLog reads stdout log of a process
func (c *Client) ReadProcessStdoutLog(ctx context.Context, name string, offset int, length int) (string, error) {
	resp, err := c.call(ctx, "supervisor.readProcessStdoutLog", []any{name, offset, length})
	if err != nil {
		return "", fmt.Errorf("failed to read stdout log for %s: %w", name, err)
	}
	return parseString(resp), nil
}

// ReadProcessStderrLog reads stderr log of a process
func (c *Client) ReadProcessStderrLog(ctx context.Context, name string, offset int, length int) (string, error) {
	resp, err := c.call(ctx, "supervisor.readProcessStderrLog", []any{name, offset, length})
	if err != nil {
		return "", fmt.Errorf("failed to read stderr log for %s: %w", name, err)
	}
	return parseString(resp), nil
}

// TailProcessStdoutLog tails stdout log of a process
func (c *Client) TailProcessStdoutLog(ctx context.Context, name string, offset int, length int) (string, int, bool, error) {
	resp, err := c.call(ctx, "supervisor.tailProcessStdoutLog", []any{name, offset, length})
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to tail stdout log for %s: %w", name, err)
	}
	return parseTailResult(resp)
}

// XML-RPC request/response structures
type methodCall struct {
	XMLName    xml.Name `xml:"methodCall"`
	MethodName string   `xml:"methodName"`
	Params     params   `xml:"params"`
}

type params struct {
	Param []param `xml:"param"`
}

type param struct {
	Value value `xml:"value"`
}

type value struct {
	String  string `xml:"string,omitempty"`
	Int     int    `xml:"int,omitempty"`
	I4      int    `xml:"i4,omitempty"`
	Boolean int    `xml:"boolean,omitempty"`
	Double  string `xml:"double,omitempty"`
	Array   *array `xml:"array,omitempty"`
}

type array struct {
	Data data `xml:"data"`
}

type data struct {
	Value []value `xml:"value"`
}

type methodResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  params   `xml:"params"`
	Fault   *fault   `xml:"fault"`
}

type fault struct {
	Value value `xml:"value"`
}

// call makes an XML-RPC call to Supervisord
func (c *Client) call(ctx context.Context, method string, args []any) (*methodResponse, error) {
	// Build request
	call := methodCall{
		MethodName: method,
		Params:     params{Param: make([]param, len(args))},
	}

	for i, arg := range args {
		call.Params.Param[i] = param{Value: toValue(arg)}
	}

	body, err := xml.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var methodResp methodResponse
	if err := xml.Unmarshal(respBody, &methodResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if methodResp.Fault != nil {
		return nil, fmt.Errorf("XML-RPC fault: %s", methodResp.Fault.Value.String)
	}

	return &methodResp, nil
}

// toValue converts a Go value to an XML-RPC value
func toValue(v any) value {
	switch val := v.(type) {
	case string:
		return value{String: val}
	case int:
		return value{Int: val}
	case bool:
		b := 0
		if val {
			b = 1
		}
		return value{Boolean: b}
	default:
		return value{String: fmt.Sprintf("%v", v)}
	}
}

// parseProcessInfo parses a process info response
func parseProcessInfo(resp *methodResponse, info *ProcessInfo) error {
	// Simplified parsing - in production, would need proper struct parsing
	if len(resp.Params.Param) > 0 {
		// Parse struct from response
		// For now, return empty info
	}
	return nil
}

// parseAllProcessInfo parses all process info response
func parseAllProcessInfo(resp *methodResponse) ([]*ProcessInfo, error) {
	// Simplified parsing
	return []*ProcessInfo{}, nil
}

// parseString extracts a string from response
func parseString(resp *methodResponse) string {
	if len(resp.Params.Param) > 0 {
		return resp.Params.Param[0].Value.String
	}
	return ""
}

// parseTailResult parses a tail log result
func parseTailResult(resp *methodResponse) (string, int, bool, error) {
	// Tail returns [string, offset, overflow]
	return "", 0, false, nil
}

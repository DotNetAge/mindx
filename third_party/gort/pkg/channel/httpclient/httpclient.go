// Package httpclient provides a generic HTTP client for channel adapters.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a generic HTTP client with common configuration.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	defaultHeaders map[string]string
}

func (c *Client) SetDefaultHeaders(headers map[string]string) {
	c.defaultHeaders = headers
}

func (c *Client) WithAuth(tokenType, token string) *Client {
	if c.defaultHeaders == nil {
		c.defaultHeaders = make(map[string]string)
	}
	c.defaultHeaders["Authorization"] = tokenType + " " + token
	return c
}

// NewClient creates a new HTTP client with the given base URL and timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
	}
}

// NewClientWithTransport creates a new HTTP client with custom transport.
func NewClientWithTransport(baseURL string, timeout time.Duration, transport *http.Transport) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
	}
}

// Post sends a POST request with JSON payload.
func (c *Client) Post(ctx context.Context, endpoint string, payload interface{}, headers map[string]string) ([]byte, error) {
	url := c.baseURL + endpoint

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.defaultHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) PostJSON(ctx context.Context, endpoint string, payload interface{}, result interface{}) error {
	respBody, err := c.Post(ctx, endpoint, payload, nil)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

// PostForm sends a POST request with form data.
func (c *Client) PostForm(ctx context.Context, endpoint string, formData map[string]string, headers map[string]string) ([]byte, error) {
	reqURL := c.baseURL + endpoint

	values := url.Values{}
	for key, value := range formData {
		values.Set(key, value)
	}
	encoded := values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range c.defaultHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Get sends a GET request.
func (c *Client) Get(ctx context.Context, endpoint string, headers map[string]string) ([]byte, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range c.defaultHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Do sends an HTTP request with the given method and returns the response body.
func (c *Client) Do(ctx context.Context, method, endpoint string, body []byte, headers map[string]string) ([]byte, error) {
	url := c.baseURL + endpoint

	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range c.defaultHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// SetHTTPClient sets a custom HTTP client.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// CloseIdleConnections closes any idle connections.
func (c *Client) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}

// Copyright 2022 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	pathLogLevel = "%s/api/v1/log-level"

	//nolint:godot
	// TODO: implement endpoints
	// pathFeed           = "%s/api/v1/user/feed"
	// pathVersion        = "%s/version"
)

// Common client errors for easier error handling.
var (
	// ErrUnauthorized indicates invalid or missing authentication credentials.
	ErrUnauthorized = errors.New("unauthorized: invalid or missing authentication token")
	// ErrForbidden indicates the user lacks permission for this action.
	ErrForbidden = errors.New("forbidden: insufficient permissions for this action")
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found: the requested resource does not exist")
)

type ClientError struct {
	StatusCode int
	Message    string
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("client error %d: %s", e.StatusCode, e.Message)
}

// Is implements error matching for ClientError.
func (e *ClientError) Is(target error) bool {
	switch {
	case errors.Is(target, ErrUnauthorized):
		return e.StatusCode == http.StatusUnauthorized
	case errors.Is(target, ErrForbidden):
		return e.StatusCode == http.StatusForbidden
	case errors.Is(target, ErrNotFound):
		return e.StatusCode == http.StatusNotFound
	}
	return false
}

type client struct {
	client *http.Client
	addr   string
}

// New returns a client at the specified url.
func New(uri string) Client {
	return &client{http.DefaultClient, strings.TrimSuffix(uri, "/")}
}

// NewClient returns a client at the specified url.
func NewClient(uri string, cli *http.Client) Client {
	return &client{cli, strings.TrimSuffix(uri, "/")}
}

// SetClient sets the http.Client.
func (c *client) SetClient(client *http.Client) {
	c.client = client
}

// SetAddress sets the server address.
func (c *client) SetAddress(addr string) {
	c.addr = addr
}

// LogLevel returns the current logging level.
func (c *client) LogLevel() (*LogLevel, error) {
	out := new(LogLevel)
	uri := fmt.Sprintf(pathLogLevel, c.addr)
	err := c.get(uri, out)
	return out, err
}

// SetLogLevel sets the logging level of the server.
func (c *client) SetLogLevel(in *LogLevel) (*LogLevel, error) {
	out := new(LogLevel)
	uri := fmt.Sprintf(pathLogLevel, c.addr)
	err := c.post(uri, in, out)
	return out, err
}

//
// HTTP request helper functions.
//

// Helper function for making an http GET request.
func (c *client) get(rawURL string, out any) error {
	return c.do(rawURL, http.MethodGet, nil, out)
}

// Helper function for making an http POST request.
func (c *client) post(rawURL string, in, out any) error {
	return c.do(rawURL, http.MethodPost, in, out)
}

// Helper function for making an http PATCH request.
func (c *client) patch(rawURL string, in, out any) error {
	return c.do(rawURL, http.MethodPatch, in, out)
}

// Helper function for making an http DELETE request.
func (c *client) delete(rawURL string) error {
	return c.do(rawURL, http.MethodDelete, nil, nil)
}

// Helper function to make an http request.
func (c *client) do(rawURL, method string, in, out any) error {
	body, err := c.open(rawURL, method, in)
	if err != nil {
		return err
	}
	defer body.Close()
	if out != nil {
		return json.NewDecoder(body).Decode(out)
	}
	return nil
}

// Helper function to open an http request.
func (c *client) open(rawURL, method string, in any) (io.ReadCloser, error) {
	uri, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, uri.String(), nil)
	if err != nil {
		return nil, err
	}
	if in != nil {
		decoded, decodeErr := json.Marshal(in)
		if decodeErr != nil {
			return nil, decodeErr
		}
		buf := bytes.NewBuffer(decoded)
		req.Body = io.NopCloser(buf)
		req.ContentLength = int64(len(decoded))
		req.Header.Set("Content-Length", strconv.Itoa(len(decoded)))
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle error status codes with specific messages
	if resp.StatusCode > http.StatusPartialContent {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))

		return nil, &ClientError{
			StatusCode: resp.StatusCode,
			Message:    formatErrorMessage(resp.StatusCode, msg),
		}
	}

	// Check if the response is HTML instead of JSON (e.g., SPA fallback, login page)
	// This provides a clearer error than "invalid character '<' looking for beginning of value"
	contentType := resp.Header.Get("Content-Type")
	if isHTMLContentType(contentType) {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, &ClientError{
			StatusCode: resp.StatusCode,
			Message:    detectHTMLErrorCause(string(body), c.addr),
		}
	}

	return resp.Body, nil
}

// isHTMLContentType checks if the content-type indicates an HTML response.
func isHTMLContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "text/html")
}

// formatErrorMessage creates a user-friendly error message based on HTTP status code.
func formatErrorMessage(statusCode int, serverMsg string) string {
	switch statusCode {
	case http.StatusUnauthorized:
		if serverMsg != "" && !strings.HasPrefix(serverMsg, "<") {
			return fmt.Sprintf("authentication failed: %s. Check that your token is valid and not expired", serverMsg)
		}
		return "authentication failed: invalid or expired token. Generate a new token in the Crow web UI under User Settings > Access Tokens"

	case http.StatusForbidden:
		if serverMsg != "" && !strings.HasPrefix(serverMsg, "<") {
			return fmt.Sprintf("permission denied: %s", serverMsg)
		}
		return "permission denied: you don't have access to this resource. Check that your token has the required scopes"

	case http.StatusNotFound:
		return "resource not found: the repository, pipeline, or resource does not exist or you don't have access to it"

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Sprintf("server unavailable (HTTP %d): the Crow server may be down or unreachable. Check the server URL and try again", statusCode)

	default:
		if serverMsg != "" && !strings.HasPrefix(serverMsg, "<") {
			return serverMsg
		}
		return fmt.Sprintf("server returned HTTP %d", statusCode)
	}
}

// detectHTMLErrorCause analyzes an HTML response to determine the likely cause.
func detectHTMLErrorCause(html string, serverAddr string) string {
	htmlLower := strings.ToLower(html)

	// Check for common login page indicators
	loginIndicators := []string{
		"login", "sign in", "signin", "log in", "authenticate",
		"<form", "password", "username", "oauth", "authorization",
	}
	isLoginPage := false
	for _, indicator := range loginIndicators {
		if strings.Contains(htmlLower, indicator) {
			isLoginPage = true
			break
		}
	}

	if isLoginPage {
		return "received login page instead of API response - your token is likely invalid or expired. Generate a new token in the Crow web UI (User Settings > Access Tokens)"
	}

	// Check for Crow SPA (likely wrong API path or server misconfiguration)
	if strings.Contains(htmlLower, "crow") || strings.Contains(html, "app.") {
		return fmt.Sprintf("received web UI instead of API response - the API endpoint may not exist or server URL is wrong (current: %s). Verify your server URL matches your Crow server version", serverAddr)
	}

	// Generic HTML response
	return "received HTML instead of JSON - check that CROW_SERVER points to your Crow instance (not a proxy or wrong URL)"
}

// mapValues converts a map to `url.Values`.
func mapValues(params map[string]string) url.Values {
	values := url.Values{}
	for key, val := range params {
		values.Add(key, val)
	}
	return values
}

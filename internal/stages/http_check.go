// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stages

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
)

// Package-level variable: detected once at startup
var isAWS bool

func init() {
	isAWS = detectAWSEnvironment()
}

// HttpStageCheck represents an HTTP-based stage check.
type HttpStageCheck schema.StageChecksHttpConfig

// Run executes the HTTP stage check by sending a GET request to the specified URL.
// It validates the response status code and content based on the check configuration.
func (c HttpStageCheck) Run(ctx context.Context, cfg schema.QuartzConfig) error {
	// Use a custom DNS resolver to avoid systemd-resolved (127.0.0.53) negative
	// caching issues. When the HTTP check starts before DNS records propagate, the
	// local stub resolver caches NXDOMAIN responses and blocks retries from
	// succeeding even after the record exists. Using the VPC resolver (169.254.169.253)
	// with a short timeout avoids this problem on AWS; falls back to Google DNS.
	resolver := &net.Resolver{
		PreferGo: true,
		Dial:     selectDNSDialer(),
	}

	tr := &http.Transport{
		// lgtm[go/disabled-certificate-check]
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.Insecure}, // #nosec G402
		DialContext: (&net.Dialer{
			Timeout:  10 * time.Second,
			Resolver: resolver,
		}).DialContext,
	}
	client := &http.Client{Transport: tr}

	url := c.formatUrl(cfg)

	log.Debug("Starting HTTP check", "url", url)
	res, err := client.Get(url)
	if err != nil {
		log.Debug("Error on HTTP check", "url", url, "err", err)
		return err
	}

	statusMatched, statusErr := c.checkResponseStatus(url, res)
	contentMatched, contentErr := c.checkResponseContent(url, res)

	if statusMatched && contentMatched {
		return nil
	}

	log.Debug("HTTP check failed", "url", url, "status", res.StatusCode, "statusErr", statusErr, "contentErr", contentErr)
	return fmt.Errorf("check failed for url %s, status %d", c.Url, res.StatusCode)
}

// Id returns the unique identifier of the HTTP stage check.
// If a URL is provided, it is used as the identifier; otherwise, the path is used.
func (c HttpStageCheck) Id() string {
	if c.Url != "" {
		return c.Url
	}

	return c.Path
}

// Type returns the type of the stage check, which is "http".
func (c HttpStageCheck) Type() string {
	return "http"
}

// RetryOpts returns the retry configuration for the HTTP stage check.
// It includes the retry limit and wait time between retries.
func (c HttpStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	r := c.Retry.Limit
	if r <= 0 {
		r = math.MaxInt // go forever if not specified
	}

	w := c.Retry.WaitSeconds
	if w <= 0 {
		w = 5 // 5 second default
	}

	return schema.StageChecksRetryConfig{
		Limit:       r,
		WaitSeconds: w,
	}
}

// formatUrl constructs the full URL for the HTTP stage check based on the configuration.
// If no URL is provided, it generates a default URL using the application name and domain.
func (c HttpStageCheck) formatUrl(cfg schema.QuartzConfig) string {
	url := c.Url
	if len(url) == 0 {
		// TODO: move this somewhere reusable
		baseUrl := fmt.Sprintf("https://%s.%s", c.App, cfg.Dns.Domain)
		if c.App == "keycloak" {
			baseUrl = fmt.Sprintf("https://keycloak.auth.%s", cfg.Dns.Domain)
		}
		url = baseUrl + c.Path
	}

	return url
}

// checkResponseContent validates the response content against the expected value or JSON key.
// Returns true if the content matches, otherwise returns false with an error.
func (c HttpStageCheck) checkResponseContent(url string, res *http.Response) (bool, error) {
	if len(c.Content.Value) == 0 && len(c.Content.Json.Key) == 0 {
		// content match not requested for this check, assume true
		return true, nil
	}

	defer res.Body.Close()
	content, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	if len(c.Content.Value) > 0 && strings.EqualFold(string(content), c.Content.Value) {
		log.Debug("HTTP check content literal matched", "url", url, "content", c.Content.Value)
		return true, nil
	}

	if len(c.Content.Json.Key) > 0 {
		var j map[string]interface{}
		err = json.Unmarshal(content, &j)
		if err != nil {
			return false, err
		}

		actual := fmt.Sprintf("%v", j[c.Content.Json.Key])
		if strings.EqualFold(actual, c.Content.Value) {
			log.Debug("HTTP check content json matched", "url", url, "content", c.Content.Value)
			return true, nil
		}

		log.Debug("HTTP check content failed match", "url", url, "expected", c.Content.Value, "found", actual)
	}

	return false, fmt.Errorf("HTTP content check failed, %s", content)
}

// checkResponseStatus validates the response status code against the expected status codes.
// Returns true if the status code matches, otherwise returns false with an error.
func (c HttpStageCheck) checkResponseStatus(url string, res *http.Response) (bool, error) {
	statusCodes := c.StatusCodes
	if len(statusCodes) == 0 {
		statusCodes = []int{200} // default to 200 if nothing provided
	}

	for _, s := range statusCodes {
		if s == res.StatusCode {
			log.Debug("HTTP check status code matched", "url", url, "status", res.StatusCode)
			return true, nil
		}
	}

	return false, fmt.Errorf("HTTP status code check failed, %d %s", res.StatusCode, res.Status)
}

// selectDNSDialer returns the appropriate DNS resolver based on environment
func selectDNSDialer() func(ctx context.Context, network, address string) (net.Conn, error) {
	if isAWS {
		return awsDNSDialer
	}
	return googleDNSDialer
}

func awsDNSDialer(ctx context.Context, network, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.DialContext(ctx, "udp", "169.254.169.253:53")
}

func googleDNSDialer(ctx context.Context, network, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.DialContext(ctx, "udp", "8.8.8.8:53")
}

// detectAWSEnvironment checks if running on AWS
func detectAWSEnvironment() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/latest/meta-data/", nil)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

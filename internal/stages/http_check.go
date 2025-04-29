package stages

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
)

// HttpStageCheck represents an HTTP-based stage check.
type HttpStageCheck schema.StageChecksHttpConfig

// Run executes the HTTP stage check by sending a GET request to the specified URL.
// It validates the response status code and content based on the check configuration.
func (c HttpStageCheck) Run(ctx context.Context, cfg schema.QuartzConfig) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.Verify}, // #nosec G402
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

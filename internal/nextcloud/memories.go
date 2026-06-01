package nextcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

// OCSCapabilities contains the subset of Nextcloud capability metadata needed by
// the importer discovery flow.
type OCSCapabilities struct {
	VersionString string
	Edition       string
	ProductName   string
}

// MemoriesDescribe contains the public API description exposed by Memories.
type MemoriesDescribe struct {
	Version      string  `json:"version"`
	BaseURL      string  `json:"baseUrl"`
	LoginFlowURL string  `json:"loginFlowUrl"`
	UID          *string `json:"uid"`
}

// MemoriesConfig contains the subset of user config needed to scope imports and
// explain source-side limitations.
type MemoriesConfig struct {
	Version                  string `json:"version"`
	TimelinePath             string `json:"timeline_path"`
	FoldersPath              string `json:"folders_path"`
	AlbumsEnabled            bool   `json:"albums_enabled"`
	SystemTagsEnabled        bool   `json:"systemtags_enabled"`
	PreviewGeneratorEnabled  bool   `json:"preview_generator_enabled"`
	RecognizeInstalled       bool   `json:"recognize_installed"`
	RecognizeEnabled         bool   `json:"recognize_enabled"`
	FaceRecognitionInstalled bool   `json:"facerecognition_installed"`
	FaceRecognitionEnabled   bool   `json:"facerecognition_enabled"`
}

// MemoriesDiscovery combines the stable Nextcloud and app-specific Memories
// information needed by the discovery-only flow.
type MemoriesDiscovery struct {
	BaseURL       string
	DAVRoot       string
	Capabilities  OCSCapabilities
	Describe      MemoriesDescribe
	Config        MemoriesConfig
	TimelineRoots []string
}

type ocsEnvelope[T any] struct {
	OCS struct {
		Meta ocsMeta `json:"meta"`
		Data T       `json:"data"`
	} `json:"ocs"`
}

type ocsMeta struct {
	Status     string `json:"status"`
	StatusCode int    `json:"statuscode"`
	Message    string `json:"message"`
}

// GetOCSCapabilities validates OCS access and returns a small subset of server
// metadata useful in discovery output.
func GetOCSCapabilities(ctx context.Context, client *Client) (*OCSCapabilities, error) {
	req, err := client.NewOCSRequest(ctx, http.MethodGet, "cloud/capabilities?format=json", nil)
	if err != nil {
		return nil, err
	}

	type capabilitiesData struct {
		Version struct {
			String      string `json:"string"`
			Edition     string `json:"edition"`
			ProductName string `json:"productname"`
		} `json:"version"`
	}

	var envelope ocsEnvelope[capabilitiesData]
	if err := doJSON(req, client, &envelope); err != nil {
		return nil, err
	}
	if envelope.OCS.Meta.StatusCode != 100 || !strings.EqualFold(envelope.OCS.Meta.Status, "ok") {
		return nil, fmt.Errorf("OCS capabilities request failed: %s (%d)", envelope.OCS.Meta.Message, envelope.OCS.Meta.StatusCode)
	}

	return &OCSCapabilities{
		VersionString: envelope.OCS.Data.Version.String,
		Edition:       envelope.OCS.Data.Version.Edition,
		ProductName:   envelope.OCS.Data.Version.ProductName,
	}, nil
}

// DescribeMemories fetches the public API description exposed by the Memories app.
func DescribeMemories(ctx context.Context, client *Client) (*MemoriesDescribe, error) {
	req, err := client.NewMemoriesRequest(ctx, http.MethodGet, "api/describe", nil)
	if err != nil {
		return nil, err
	}

	var describe MemoriesDescribe
	if err := doJSON(req, client, &describe); err != nil {
		return nil, err
	}
	if describe.Version == "" {
		return nil, errors.New("Memories describe response did not include a version")
	}
	return &describe, nil
}

// GetMemoriesConfig fetches the current user's Memories configuration.
func GetMemoriesConfig(ctx context.Context, client *Client) (*MemoriesConfig, error) {
	req, err := client.NewMemoriesRequest(ctx, http.MethodGet, "api/config", nil)
	if err != nil {
		return nil, err
	}

	var config MemoriesConfig
	if err := doJSON(req, client, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// DiscoverMemories collects both OCS and Memories app metadata to drive the
// discovery-only command flow.
func DiscoverMemories(ctx context.Context, client *Client) (*MemoriesDiscovery, error) {
	capabilities, err := GetOCSCapabilities(ctx, client)
	if err != nil {
		return nil, err
	}
	describe, err := DescribeMemories(ctx, client)
	if err != nil {
		return nil, err
	}
	config, err := GetMemoriesConfig(ctx, client)
	if err != nil {
		return nil, err
	}

	timelineRoots := splitTimelineRoots(config.TimelinePath)
	if len(timelineRoots) == 0 {
		return nil, errors.New("Memories config did not provide any timeline roots")
	}

	return &MemoriesDiscovery{
		BaseURL:       client.BaseURL(),
		DAVRoot:       client.DAVRoot(),
		Capabilities:  *capabilities,
		Describe:      *describe,
		Config:        *config,
		TimelineRoots: timelineRoots,
	}, nil
}

func splitTimelineRoots(raw string) []string {
	parts := strings.Split(raw, ";")
	roots := make([]string, 0, len(parts))

	for _, part := range parts {
		root := strings.TrimSpace(part)
		root = strings.TrimRight(root, "/")
		if root == "" {
			continue
		}
		if !strings.HasPrefix(root, "/") {
			root = "/" + root
		}
		if slices.Contains(roots, root) {
			continue
		}
		roots = append(roots, root)
	}

	return roots
}

func doJSON(req *http.Request, client *Client, target any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("request %s %s failed with status %d: %s", req.Method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(body)))
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}

	return nil
}

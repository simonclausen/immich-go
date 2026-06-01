package nextcloud

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOCSCapabilities(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ocs/v2.php/cloud/capabilities":
			assert.Equal(t, "true", r.Header.Get("OCS-APIRequest"))
			assert.Equal(t, "json", r.URL.Query().Get("format"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ocs":{"meta":{"status":"ok","statuscode":100,"message":"OK"},"data":{"version":{"string":"31.0.0","edition":"community","productname":"Nextcloud"}}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL)
	capabilities, err := GetOCSCapabilities(context.Background(), client)
	require.NoError(t, err)
	assert.Equal(t, "31.0.0", capabilities.VersionString)
	assert.Equal(t, "community", capabilities.Edition)
	assert.Equal(t, "Nextcloud", capabilities.ProductName)
}

func TestDiscoverMemories(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ocs/v2.php/cloud/capabilities":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ocs":{"meta":{"status":"ok","statuscode":100,"message":"OK"},"data":{"version":{"string":"31.0.0","edition":"community","productname":"Nextcloud"}}}}`)
		case "/index.php/apps/memories/api/describe":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"version":"7.5.0","baseUrl":"https://cloud.example.com/apps/memories","loginFlowUrl":"https://cloud.example.com/login","uid":"alice"}`)
		case "/index.php/apps/memories/api/config":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"version":"7.5.0","timeline_path":"/Photos;/Scans/;/Photos","folders_path":"/","albums_enabled":true,"systemtags_enabled":true,"preview_generator_enabled":false,"recognize_installed":false,"recognize_enabled":false,"facerecognition_installed":false,"facerecognition_enabled":false}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL)
	discovery, err := DiscoverMemories(context.Background(), client)
	require.NoError(t, err)

	assert.Equal(t, server.URL, discovery.BaseURL)
	assert.Equal(t, server.URL+"/remote.php/dav", discovery.DAVRoot)
	assert.Equal(t, "31.0.0", discovery.Capabilities.VersionString)
	assert.Equal(t, "7.5.0", discovery.Describe.Version)
	assert.Equal(t, []string{"/Photos", "/Scans"}, discovery.TimelineRoots)
	assert.True(t, discovery.Config.AlbumsEnabled)
	assert.True(t, discovery.Config.SystemTagsEnabled)
	assert.Equal(t, "/", discovery.Config.FoldersPath)
	if assert.NotNil(t, discovery.Describe.UID) {
		assert.Equal(t, "alice", *discovery.Describe.UID)
	}
}

func TestDiscoverMemoriesRejectsEmptyTimelineRoots(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ocs/v2.php/cloud/capabilities":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ocs":{"meta":{"status":"ok","statuscode":100,"message":"OK"},"data":{"version":{"string":"31.0.0","edition":"community","productname":"Nextcloud"}}}}`)
		case "/index.php/apps/memories/api/describe":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"version":"7.5.0","baseUrl":"https://cloud.example.com/apps/memories","loginFlowUrl":"https://cloud.example.com/login","uid":"alice"}`)
		case "/index.php/apps/memories/api/config":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"version":"7.5.0","timeline_path":"","folders_path":"/","albums_enabled":true,"systemtags_enabled":true,"preview_generator_enabled":false}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL)
	_, err := DiscoverMemories(context.Background(), client)
	require.Error(t, err)
	assert.ErrorContains(t, err, "timeline roots")
}

func TestGetOCSCapabilitiesRejectsBadMetaStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ocs":{"meta":{"status":"failure","statuscode":997,"message":"Authentication failed"},"data":{}}}`)
	}))
	defer server.Close()

	client := mustNewClient(t, server.URL)
	_, err := GetOCSCapabilities(context.Background(), client)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Authentication failed")
}

func mustNewClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	client, err := NewClient(Config{
		BaseURL:  baseURL,
		Username: "alice",
		Password: "secret",
		Timeout:  time.Minute,
	})
	require.NoError(t, err)
	return client
}

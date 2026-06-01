package nextcloudmemories

import (
	"bytes"
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/nextcloud"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromNextcloudMemoriesCommandMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parent := &cobra.Command{Use: "upload"}
	a := app.New(ctx, parent)
	cmd := NewFromNextcloudMemoriesCommand(ctx, parent, a, nil)

	assert.Equal(t, "from-nextcloud-memories [flags]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "from-nc-memories")
	assert.True(t, cmd.Hidden)
	assert.Contains(t, cmd.Long, "not arbitrary Nextcloud storage paths")
	assert.Contains(t, cmd.Example, "--discover-only")
	assert.Contains(t, cmd.Example, "--timeline-root=/Photos")
	assert.NotNil(t, cmd.Flag("nextcloud-url"))
	assert.NotNil(t, cmd.Flag("nextcloud-user"))
	assert.NotNil(t, cmd.Flag("nextcloud-password"))
	assert.NotNil(t, cmd.Flag("discover-only"))
	assert.NotNil(t, cmd.Flag("timeline-root"))
	assert.NotNil(t, cmd.Flag("sync-albums"))
	assert.NotNil(t, cmd.Flag("allow-unindexed"))
}

func TestCommandValidate(t *testing.T) {
	t.Parallel()

	t.Run("missing required flags", func(t *testing.T) {
		t.Parallel()

		nc := &Command{NextcloudClientTimeout: 5 * time.Minute}
		err := nc.validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "--nextcloud-url")
		assert.ErrorContains(t, err, "--nextcloud-user")
		assert.ErrorContains(t, err, "--nextcloud-password")
	})

	t.Run("normalizes relative timeline root", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           "https://cloud.example.com/",
			NextcloudUser:          "alice",
			NextcloudPassword:      "secret",
			NextcloudClientTimeout: 5 * time.Minute,
			TimelineRoots:          []string{"Photos"},
		}
		err := nc.validate()
		require.NoError(t, err)
		assert.Equal(t, []string{"/Photos"}, nc.TimelineRoots)
	})

	t.Run("normalizes url and roots", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           " https://cloud.example.com/remote.php/ ",
			NextcloudUser:          " alice ",
			NextcloudPassword:      " secret ",
			NextcloudClientTimeout: 5 * time.Minute,
			TimelineRoots:          []string{" //Photos/ ", "/Photos", "/Scans//"},
		}
		err := nc.validate()
		require.NoError(t, err)
		assert.Equal(t, "https://cloud.example.com/remote.php", nc.NextcloudURL)
		assert.Equal(t, "alice", nc.NextcloudUser)
		assert.Equal(t, "secret", nc.NextcloudPassword)
		assert.Equal(t, []string{"/Photos", "/Scans"}, nc.TimelineRoots)
	})

	t.Run("invalid timeout", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           "https://cloud.example.com",
			NextcloudUser:          "alice",
			NextcloudPassword:      "secret",
			NextcloudClientTimeout: 0,
		}
		err := nc.validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "--nextcloud-client-timeout")
	})
}

func TestCommandRunRequiresRunner(t *testing.T) {
	t.Parallel()

	nc := Command{
		NextcloudURL:           "https://cloud.example.com",
		NextcloudUser:          "alice",
		NextcloudPassword:      "secret",
		NextcloudClientTimeout: 5 * time.Minute,
	}

	err := nc.Run(&cobra.Command{}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "runner is not configured")
}

func TestCommandRunImportUsesRunner(t *testing.T) {
	t.Parallel()

	nc := Command{
		NextcloudURL:           "https://cloud.example.com",
		NextcloudUser:          "alice",
		NextcloudPassword:      "secret",
		NextcloudClientTimeout: 5 * time.Minute,
		app:                    newTestApp(t),
		sourceFS: fstest.MapFS{
			"Photos/IMG_0001.JPG": {Data: []byte("image")},
		},
		selectedRoots: []string{"/Photos"},
	}

	called := false
	err := nc.Run(&cobra.Command{}, runnerFunc(func(cmd *cobra.Command, adapter adapters.Reader) error {
		called = true
		groups := collectGroups(adapter.Browse(context.Background()))
		require.Len(t, groups, 1)
		assert.Equal(t, "Photos/IMG_0001.JPG", groups[0].Assets[0].File.Name())
		return nil
	}))
	require.NoError(t, err)
	assert.True(t, called)
}

func TestCommandRunDiscoverOnly(t *testing.T) {
	t.Parallel()

	nc := Command{
		NextcloudURL:           "https://cloud.example.com",
		NextcloudUser:          "alice",
		NextcloudPassword:      "secret",
		NextcloudClientTimeout: 5 * time.Minute,
		DiscoverOnly:           true,
		discover: func(ctx context.Context, cfg nextcloud.Config) (*nextcloud.MemoriesDiscovery, error) {
			assert.Equal(t, "https://cloud.example.com", cfg.BaseURL)
			assert.Equal(t, "alice", cfg.Username)
			assert.Equal(t, "secret", cfg.Password)
			return &nextcloud.MemoriesDiscovery{
				BaseURL:       cfg.BaseURL,
				DAVRoot:       cfg.BaseURL + "/remote.php/dav",
				Capabilities:  nextcloud.OCSCapabilities{VersionString: "31.0.0", ProductName: "Nextcloud", Edition: "community"},
				Describe:      nextcloud.MemoriesDescribe{Version: "7.5.0", BaseURL: cfg.BaseURL + "/index.php/apps/memories", UID: stringPtr("alice")},
				Config:        nextcloud.MemoriesConfig{FoldersPath: "/", AlbumsEnabled: true, SystemTagsEnabled: true, PreviewGeneratorEnabled: false},
				TimelineRoots: []string{"/Photos", "/Scans"},
			}, nil
		},
	}

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := nc.Run(cmd, nil)
	require.NoError(t, err)
	text := output.String()
	assert.Contains(t, text, "Nextcloud Memories scaffold")
	assert.Contains(t, text, "Nextcloud Memories discovery")
	assert.Contains(t, text, "nextcloud-version: 31.0.0")
	assert.Contains(t, text, "configured-timeline-roots:")
	assert.Contains(t, text, "selected-timeline-roots:")
	assert.Contains(t, text, "    - /Photos")
	assert.Contains(t, text, "albums-enabled: true")
}

func TestCommandRunDiscoverOnlyRejectsUnknownRequestedRoots(t *testing.T) {
	t.Parallel()

	nc := Command{
		NextcloudURL:           "https://cloud.example.com",
		NextcloudUser:          "alice",
		NextcloudPassword:      "secret",
		NextcloudClientTimeout: 5 * time.Minute,
		DiscoverOnly:           true,
		TimelineRoots:          []string{"/Scans"},
		discover: func(ctx context.Context, cfg nextcloud.Config) (*nextcloud.MemoriesDiscovery, error) {
			return &nextcloud.MemoriesDiscovery{TimelineRoots: []string{"/Photos"}}, nil
		},
	}

	err := nc.Run(&cobra.Command{}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "requested --timeline-root")
	assert.ErrorContains(t, err, "/Photos")
}

func TestCommandIntentSummary(t *testing.T) {
	t.Parallel()

	t.Run("auto discover roots", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:      "https://cloud.example.com",
			NextcloudUser:     "alice",
			DiscoverOnly:      true,
			SyncAlbums:        true,
			AllowUnindexed:    false,
			TimelineRoots:     nil,
			NextcloudPassword: "secret",
		}

		summary := nc.intentSummary()
		assert.Contains(t, summary, "mode: discover-only")
		assert.Contains(t, summary, "timeline-roots: all configured Memories timeline roots")
		assert.Contains(t, summary, "sync-albums: true")
		assert.NotContains(t, summary, "secret")
	})

	t.Run("restricted roots", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           "https://cloud.example.com",
			NextcloudUser:          "alice",
			SyncAlbums:             false,
			AllowUnindexed:         true,
			NextcloudSkipVerifySSL: true,
			TimelineRoots:          []string{"/Photos", "/Scans"},
		}

		summary := nc.intentSummary()
		assert.Contains(t, summary, "mode: import")
		assert.Contains(t, summary, "timeline-roots: /Photos, /Scans")
		assert.Contains(t, summary, "sync-albums: false")
		assert.Contains(t, summary, "allow-unindexed: true")
		assert.Contains(t, summary, "skip-verify-ssl: true")
	})
}

func TestCommandRejectsPositionalArguments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parent := &cobra.Command{Use: "upload"}
	a := app.New(ctx, parent)
	_ = NewFromNextcloudMemoriesCommand(ctx, parent, a, nil)

	err := cobra.NoArgs(&cobra.Command{Use: "from-nextcloud-memories"}, []string{"unexpected-path"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown command \"unexpected-path\" for \"from-nextcloud-memories\"")
}

func stringPtr(value string) *string {
	return &value
}

type runnerFunc func(cmd *cobra.Command, adapter adapters.Reader) error

func (fn runnerFunc) Run(cmd *cobra.Command, adapter adapters.Reader) error {
	return fn(cmd, adapter)
}

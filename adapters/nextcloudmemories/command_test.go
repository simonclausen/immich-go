package nextcloudmemories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/simulot/immich-go/app"
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

	t.Run("invalid timeline root", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           "https://cloud.example.com/",
			NextcloudUser:          "alice",
			NextcloudPassword:      "secret",
			NextcloudClientTimeout: 5 * time.Minute,
			TimelineRoots:          []string{"Photos"},
		}
		err := nc.validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "must start with /")
	})

	t.Run("normalizes url and roots", func(t *testing.T) {
		t.Parallel()

		nc := &Command{
			NextcloudURL:           " https://cloud.example.com/remote.php/ ",
			NextcloudUser:          " alice ",
			NextcloudPassword:      " secret ",
			NextcloudClientTimeout: 5 * time.Minute,
			TimelineRoots:          []string{" /Photos/ ", "/Photos", "/Scans/"},
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

func TestCommandRunReturnsExplicitNotImplemented(t *testing.T) {
	t.Parallel()

	base := Command{
		NextcloudURL:           "https://cloud.example.com",
		NextcloudUser:          "alice",
		NextcloudPassword:      "secret",
		NextcloudClientTimeout: 5 * time.Minute,
	}

	t.Run("discover only", func(t *testing.T) {
		t.Parallel()

		nc := base
		nc.DiscoverOnly = true
		err := nc.Run(&cobra.Command{}, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotImplemented))
		assert.ErrorContains(t, err, "source discovery")
	})

	t.Run("import path", func(t *testing.T) {
		t.Parallel()

		nc := base
		err := nc.Run(&cobra.Command{}, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotImplemented))
		assert.ErrorContains(t, err, "asset browsing")
	})
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

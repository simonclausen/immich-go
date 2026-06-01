package nextcloudmemories

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/fileprocessor"
	"github.com/simulot/immich-go/internal/nextcloud"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowseEnumeratesSupportedAssets(t *testing.T) {
	t.Parallel()

	nc := &Command{
		app: newTestApp(t),
		sourceFS: fstest.MapFS{
			"Photos/IMG_0001.JPG":      {Data: []byte("image"), ModTime: time.Unix(1700000000, 0)},
			"Photos/VID_0001.MP4":      {Data: []byte("video"), ModTime: time.Unix(1700000100, 0)},
			"Photos/ignored.json":      {Data: []byte("{}")},
			"Photos/readme.txt":        {Data: []byte("notes")},
			"Photos/.DS_Store":         {Data: []byte("finder")},
			"Photos/Sub/IMG_0002.HEIC": {Data: []byte("heic")},
			"Scans/SCAN_0001.JPG":      {Data: []byte("scan")},
		},
		selectedRoots: []string{"/Photos"},
	}

	groups := collectGroups(nc.Browse(context.Background()))
	require.Len(t, groups, 3)

	assetNames := []string{}
	for _, group := range groups {
		require.Len(t, group.Assets, 1)
		assetNames = append(assetNames, group.Assets[0].File.Name())
	}

	assert.ElementsMatch(t, []string{
		"Photos/IMG_0001.JPG",
		"Photos/VID_0001.MP4",
		"Photos/Sub/IMG_0002.HEIC",
	}, assetNames)

	counts := nc.app.FileProcessor().Logger().GetCounts()
	assert.EqualValues(t, 2, counts[fileevent.DiscoveredImage])
	assert.EqualValues(t, 1, counts[fileevent.DiscoveredVideo])
	assert.EqualValues(t, 1, counts[fileevent.DiscoveredSidecar])
	assert.EqualValues(t, 1, counts[fileevent.DiscoveredUnknown])
	assert.EqualValues(t, 1, counts[fileevent.DiscoveredBanned])
}

func TestBrowseDeduplicatesOverlappingRoots(t *testing.T) {
	t.Parallel()

	nc := &Command{
		app: newTestApp(t),
		sourceFS: fstest.MapFS{
			"Photos/Trips/IMG_0001.JPG": {Data: []byte("image")},
		},
		selectedRoots: []string{"/Photos", "/Photos/Trips"},
	}

	groups := collectGroups(nc.Browse(context.Background()))
	require.Len(t, groups, 1)
	assert.Equal(t, "Photos/Trips/IMG_0001.JPG", groups[0].Assets[0].File.Name())

	counts := nc.app.FileProcessor().Logger().GetCounts()
	assert.EqualValues(t, 1, counts[fileevent.DiscoveredImage])
}

func TestTimelineRootToFSPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".", timelineRootToFSPath("/"))
	assert.Equal(t, ".", timelineRootToFSPath(" "))
	assert.Equal(t, "Photos", timelineRootToFSPath("/Photos"))
}

func TestBrowsePrefersSearchWhenAvailable(t *testing.T) {
	t.Parallel()

	searchFS := &fakeSearchFS{
		MapFS: fstest.MapFS{
			"Photos/Trips/IMG_0001.JPG": {Data: []byte("image")},
			"Photos/Trips/IMG_0002.JPG": {Data: []byte("image")},
		},
		entries: []nextcloud.SearchEntry{
			{Path: "Photos/Trips/IMG_0001.JPG", Info: fakeSearchInfo{name: "IMG_0001.JPG", size: 5}},
			{Path: "Photos/Trips/IMG_0002.JPG", Info: fakeSearchInfo{name: "IMG_0002.JPG", size: 5}},
		},
	}

	nc := &Command{
		app:           newTestApp(t),
		sourceFS:      searchFS,
		selectedRoots: []string{"/Photos"},
	}

	groups := collectGroups(nc.Browse(context.Background()))
	require.Len(t, groups, 2)
	assert.Equal(t, []string{"Photos"}, searchFS.scopes)

	counts := nc.app.FileProcessor().Logger().GetCounts()
	assert.EqualValues(t, 2, counts[fileevent.DiscoveredImage])
}

type fakeSearchFS struct {
	fstest.MapFS
	entries []nextcloud.SearchEntry
	scopes  []string
	err     error
}

func (f *fakeSearchFS) SearchFiles(scope string) ([]nextcloud.SearchEntry, error) {
	f.scopes = append(f.scopes, scope)
	if f.err != nil {
		return nil, f.err
	}
	return append([]nextcloud.SearchEntry(nil), f.entries...), nil
}

type fakeSearchInfo struct {
	name string
	size int64
}

func (f fakeSearchInfo) Name() string       { return f.name }
func (f fakeSearchInfo) Size() int64        { return f.size }
func (f fakeSearchInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeSearchInfo) ModTime() time.Time { return time.Unix(1700000000, 0) }
func (f fakeSearchInfo) IsDir() bool        { return false }
func (f fakeSearchInfo) Sys() any           { return nil }

func collectGroups(in chan *assets.Group) []*assets.Group {
	groups := []*assets.Group{}
	for group := range in {
		groups = append(groups, group)
	}
	return groups
}

func newTestApp(t *testing.T) *app.Application {
	t.Helper()

	a := app.New(context.Background(), &cobra.Command{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := fileevent.NewBus()
	tracker := assettracker.NewWithBus(logger, false, bus)
	a.SetFileProcessor(fileprocessor.NewWithBus(tracker, logger, bus))
	return a
}

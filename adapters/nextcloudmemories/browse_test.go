package nextcloudmemories

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/simulot/immich-go/internal/fileprocessor"
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

func TestBrowseEnrichesAssetsWithMemoriesMetadata(t *testing.T) {
	t.Parallel()

	nc := &Command{
		app: newTestApp(t),
		sourceFS: fstest.MapFS{
			"Photos/IMG_0001.JPG": {Data: []byte("image"), ModTime: time.Unix(1700000000, 0)},
		},
		selectedRoots: []string{"/Photos"},
		metadataIndex: &memoriesMetadataIndex{
			byPath: map[string]*assets.Metadata{
				"Photos/IMG_0001.JPG": {
					Description: "Sunset",
					Favorited:   true,
					Rating:      5,
					Albums:      []assets.Album{assets.NewAlbum("", "Roadtrip", "")},
					Tags:        []assets.Tag{{Name: "Travel", Value: "Travel"}},
				},
			},
		},
	}

	groups := collectGroups(nc.Browse(context.Background()))
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Assets, 1)

	asset := groups[0].Assets[0]
	require.NotNil(t, asset.FromApplication)
	assert.Equal(t, "Sunset", asset.Description)
	assert.True(t, asset.Favorite)
	assert.Equal(t, 5, asset.Rating)
	require.Len(t, asset.Albums, 1)
	assert.Equal(t, "Roadtrip", asset.Albums[0].Title)
	require.Len(t, asset.Tags, 1)
	assert.Equal(t, "Travel", asset.Tags[0].Value)
}

func TestBrowseWarnsAndContinuesForUnindexedAssetsByDefault(t *testing.T) {
	t.Parallel()

	nc := &Command{
		app: newTestApp(t),
		sourceFS: fstest.MapFS{
			"Photos/IMG_0001.JPG": {Data: []byte("image")},
		},
		selectedRoots: []string{"/Photos"},
		metadataIndex: newMemoriesMetadataIndex(),
	}

	groups := collectGroups(nc.Browse(context.Background()))
	require.Len(t, groups, 1)
	assert.Nil(t, groups[0].Assets[0].FromApplication)
}

func TestBrowseRejectsUnindexedAssetsWhenStrictModeIsRequested(t *testing.T) {
	t.Parallel()

	nc := &Command{
		app:            newTestApp(t),
		RequireIndexed: true,
		sourceFS: fstest.MapFS{
			"Photos/IMG_0001.JPG": {Data: []byte("image")},
		},
		selectedRoots: []string{"/Photos"},
		metadataIndex: newMemoriesMetadataIndex(),
	}

	groups := collectGroups(nc.Browse(context.Background()))
	assert.Empty(t, groups)
}

func TestTimelineRootToFSPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".", timelineRootToFSPath("/"))
	assert.Equal(t, ".", timelineRootToFSPath(" "))
	assert.Equal(t, "Photos", timelineRootToFSPath("/Photos"))
}

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
	a.Log().Logger = logger
	bus := fileevent.NewBus()
	tracker := assettracker.NewWithBus(logger, false, bus)
	a.SetFileProcessor(fileprocessor.NewWithBus(tracker, logger, bus))
	return a
}

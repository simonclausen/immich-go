package nextcloudmemories

import (
	"testing"
	"time"

	"github.com/simulot/immich-go/internal/nextcloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataFromMemoriesMapsArchivedAndSharedAlbums(t *testing.T) {
	t.Parallel()

	info := &nextcloud.MemoriesImageInfo{
		Basename:  "IMG_0001.JPG",
		DateTaken: 1700000000,
		Exif: map[string]any{
			"Description":  "Sunset",
			"Rating":       "4",
			"GPSLatitude":  "12.34",
			"GPSLongitude": "56.78",
		},
		Tags: map[string]string{
			"1": "Travel",
		},
	}
	info.Clusters.Albums = []nextcloud.MemoriesAlbum{
		{Name: "Roadtrip", User: "alice"},
		{Name: "Roadtrip", User: "bob", UserDisplay: "Bob"},
	}

	md := metadataFromMemories(nextcloud.MemoriesPhoto{
		Archived:   true,
		IsFavorite: true,
		DateTaken:  1700000000,
	}, info, "alice")

	require.NotNil(t, md)
	assert.True(t, md.Archived)
	assert.True(t, md.Favorited)
	assert.Equal(t, "Sunset", md.Description)
	assert.Equal(t, byte(4), md.Rating)
	assert.Equal(t, 12.34, md.Latitude)
	assert.Equal(t, 56.78, md.Longitude)
	assert.Equal(t, time.Unix(1700000000, 0).In(time.Local), md.DateTaken)
	require.Len(t, md.Tags, 1)
	assert.Equal(t, "Travel", md.Tags[0].Value)
	require.Len(t, md.Albums, 2)
	assert.Equal(t, "Roadtrip", md.Albums[0].Title)
	assert.Equal(t, "Roadtrip (shared by Bob)", md.Albums[1].Title)
}
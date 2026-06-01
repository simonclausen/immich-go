package upload

import (
	"context"
	"testing"

	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/internal/assets"
	"github.com/simulot/immich-go/internal/assets/cache"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinishingSkipsResumeWhenPauseDisabled(t *testing.T) {
	t.Parallel()

	uc := &UpCmd{
		client: app.Client{
			PauseImmichBackgroundJobs: false,
		},
		app:         app.New(context.Background(), &cobra.Command{}),
		albumsCache: cache.NewCollectionCache(1, func(album assets.Album, ids []string) (assets.Album, error) { return album, nil }),
		tagsCache:   cache.NewCollectionCache(1, func(tag assets.Tag, ids []string) (assets.Tag, error) { return tag, nil }),
	}

	err := uc.finishing(context.Background())
	require.NoError(t, err)
	assert.True(t, uc.finished)
}

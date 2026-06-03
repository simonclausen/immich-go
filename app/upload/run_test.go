package upload

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/simulot/immich-go/adapters"
	"github.com/simulot/immich-go/app"
	"github.com/simulot/immich-go/immich"
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

func TestSaveAlbumRestoresAlbumUsers(t *testing.T) {
	t.Parallel()

	var addUsersCalls [][]immich.AlbumUserAdd
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /api/albums":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"album-1","albumName":"Roadtrip"}`))
		case http.MethodGet + " /api/albums/album-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"album-1","albumName":"Roadtrip","albumUsers":[{"role":"owner","user":{"id":"owner-1","email":"owner@example.com"}}]}`))
		case http.MethodPut + " /api/albums/album-1/users":
			var body struct {
				AlbumUsers []immich.AlbumUserAdd `json:"albumUsers"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			addUsersCalls = append(addUsersCalls, body.AlbumUsers)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"album-1","albumName":"Roadtrip"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	immichClient, err := immich.NewImmichClient(server.URL, "test-key")
	require.NoError(t, err)
	uc := &UpCmd{
		client: app.Client{
			Immich: immichClient,
		},
		adapter: albumUserProviderStub{users: []adapters.AlbumUser{{UserID: "user-2", Role: "editor"}}},
		app:     newUploadTestApp(),
	}

	album, err := uc.saveAlbum(context.Background(), assets.Album{Title: "Roadtrip"}, []string{"asset-1"})
	require.NoError(t, err)
	assert.Equal(t, "album-1", album.ID)
	require.Len(t, addUsersCalls, 1)
	assert.Equal(t, []immich.AlbumUserAdd{{UserID: "user-2", Role: immich.AlbumUserRoleEditor}}, addUsersCalls[0])
}

func TestSaveAlbumUpdatesExistingAlbumUserRole(t *testing.T) {
	t.Parallel()

	var updateCalls []albumUserUpdateCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case http.MethodPut + " /api/albums/album-1/assets":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case http.MethodGet + " /api/albums/album-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"album-1","albumName":"Roadtrip","albumUsers":[{"role":"viewer","user":{"id":"user-2","email":"viewer@example.com"}}]}`))
		case http.MethodPut + " /api/albums/album-1/user/user-2":
			var body struct {
				Role immich.AlbumUserRole `json:"role"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			updateCalls = append(updateCalls, albumUserUpdateCall{albumID: "album-1", userID: "user-2", role: body.Role})
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	immichClient, err := immich.NewImmichClient(server.URL, "test-key")
	require.NoError(t, err)
	uc := &UpCmd{
		client: app.Client{
			Immich: immichClient,
		},
		adapter: albumUserProviderStub{users: []adapters.AlbumUser{{UserID: "user-2", Role: "editor"}}},
		app:     newUploadTestApp(),
	}

	_, err = uc.saveAlbum(context.Background(), assets.Album{ID: "album-1", Title: "Roadtrip"}, []string{"asset-1"})
	require.NoError(t, err)
	require.Len(t, updateCalls, 1)
	assert.Equal(t, albumUserUpdateCall{albumID: "album-1", userID: "user-2", role: immich.AlbumUserRoleEditor}, updateCalls[0])
}

type albumUserProviderStub struct {
	users []adapters.AlbumUser
}

func (s albumUserProviderStub) Browse(context.Context) chan *assets.Group {
	return make(chan *assets.Group)
}

func (s albumUserProviderStub) DesiredAlbumUsers(context.Context, assets.Album) ([]adapters.AlbumUser, error) {
	return s.users, nil
}

type albumUserUpdateCall struct {
	albumID string
	userID  string
	role    immich.AlbumUserRole
}

func newUploadTestApp() *app.Application {
	a := app.New(context.Background(), &cobra.Command{})
	a.Log().Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return a
}

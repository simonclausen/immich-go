package nextcloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFilesParsesRecursiveResults(t *testing.T) {
	t.Parallel()

	var method string
	var contentType string
	var authUser string
	var authPassword string
	var requestBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		authUser, authPassword, _ = r.BasicAuth()
		payload, _ := io.ReadAll(r.Body)
		requestBody = string(payload)

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
  <d:response>
    <d:href>/remote.php/dav/files/alice/Photos/Trips/IMG_0001.JPG</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>IMG_0001.JPG</d:displayname>
        <d:getcontenttype>image/jpeg</d:getcontenttype>
        <d:getetag>"etag-1"</d:getetag>
        <d:getcontentlength>12345</d:getcontentlength>
        <d:getlastmodified>Wed, 20 Jul 2022 05:12:23 GMT</d:getlastmodified>
        <oc:fileid>7</oc:fileid>
        <oc:checksums>
          <oc:checksum>sha1:0123456789abcdef</oc:checksum>
        </oc:checksums>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "alice",
		Password: "secret",
		Timeout:  time.Minute,
	})
	require.NoError(t, err)

	entries, err := client.SearchFiles(context.Background(), "/files/alice/Photos")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "SEARCH", method)
	assert.Equal(t, "text/xml; charset=utf-8", contentType)
	assert.Equal(t, "alice", authUser)
	assert.Equal(t, "secret", authPassword)
	assert.Contains(t, requestBody, "<d:href>/files/alice/Photos</d:href>")
	assert.Contains(t, requestBody, "<d:is-collection/>")

	assert.Equal(t, "/files/alice/Photos/Trips/IMG_0001.JPG", entries[0].Path)
	assert.Equal(t, "7", entries[0].FileID)
	assert.Equal(t, []string{"sha1:0123456789abcdef"}, entries[0].Checksums)
	assert.Equal(t, int64(12345), entries[0].Info.Size())
	assert.Equal(t, "IMG_0001.JPG", entries[0].Info.Name())
	assert.Equal(t, time.Date(2022, time.July, 20, 5, 12, 23, 0, time.UTC), entries[0].Info.ModTime())
}

func TestSearchFilesReturnsUnsupportedFor405(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "SEARCH not allowed")
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "alice",
		Password: "secret",
		Timeout:  time.Minute,
	})
	require.NoError(t, err)

	_, err = client.SearchFiles(context.Background(), "/files/alice/Photos")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSearchUnsupported)
}

func TestBuildSearchRequestBodyEscapesScope(t *testing.T) {
	t.Parallel()

	body, err := buildSearchRequestBody(`/files/alice/Photos & Trips`)
	require.NoError(t, err)
	assert.Contains(t, string(body), `<d:href>/files/alice/Photos &amp; Trips</d:href>`)
	assert.True(t, strings.Contains(string(body), `<d:depth>infinity</d:depth>`))
}

package immich

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"testing/fstest"

	"github.com/simulot/immich-go/internal/assets"
<<<<<<< HEAD
	"github.com/simulot/immich-go/internal/fshelper"
)

=======
	"github.com/simulot/immich-go/internal/filetypes"
	"github.com/simulot/immich-go/internal/fshelper"
)

func newTestImmichClient(t *testing.T, serverURL string) *ImmichClient {
	t.Helper()

	ic, err := NewImmichClient(serverURL, "key")
	if err != nil {
		t.Fatalf("NewImmichClient() error = %v", err)
	}
	ic.supportedMediaTypes = filetypes.DefaultSupportedMedia
	return ic
}

>>>>>>> d3a63bd (fix(upload): retry transient asset upload failures)
func TestUploadAssetIgnoresClosedPipeWhenServerAlreadyAcceptedUpload(t *testing.T) {
	t.Parallel()
	"github.com/simulot/immich-go/internal/filetypes"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/assets" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-immich-checksum"); got != "checksum" {
			t.Fatalf("unexpected checksum header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"asset-id","status":"created"}`))
	ic.supportedMediaTypes = filetypes.DefaultSupportedMedia
		if !ok {
			t.Fatal("response writer does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
		}
		_ = conn.Close()
	}))
	defer server.Close()

<<<<<<< HEAD
	ic, err := NewImmichClient(server.URL, "key")
	if err != nil {
		t.Fatalf("NewImmichClient() error = %v", err)
	}
=======
	ic := newTestImmichClient(t, server.URL)
>>>>>>> d3a63bd (fix(upload): retry transient asset upload failures)
	ic.client.Timeout = 2 * time.Second

	la := &assets.Asset{
		File:             fshelper.FSName(fstest.MapFS{"photo.jpg": {Data: []byte("upload payload")}}, "photo.jpg"),
		OriginalFileName: "photo.jpg",
		FileSize:         len("upload payload"),
		Checksum:         "checksum",
		FileDate:         time.Unix(0, 0),
	ic := newTestImmichClient(t, server.URL)
	defer la.Close()

	resp, err := ic.uploadAssetOnce(context.Background(), la, EndPointAssetUpload, "")
	if err != nil {
		t.Fatalf("uploadAssetOnce() error = %v", err)
	}
	if resp.Status != UploadCreated {
		t.Fatalf("uploadAssetOnce() status = %q, want %q", resp.Status, UploadCreated)
	}
	if resp.ID != "asset-id" {
		t.Fatalf("uploadAssetOnce() id = %q, want %q", resp.ID, "asset-id")
	}
}

func TestUploadAssetRetriesTransientServerError(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		attempts int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		currentAttempt := attempts
		mu.Unlock()

		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		if currentAttempt < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"Bad Gateway","statusCode":502,"message":"upstream busy"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"asset-id","status":"created"}`))
	}))
	defer server.Close()

<<<<<<< HEAD
	ic, err := NewImmichClient(server.URL, "key")
	if err != nil {
		t.Fatalf("NewImmichClient() error = %v", err)
	}
=======
	ic := newTestImmichClient(t, server.URL)
>>>>>>> d3a63bd (fix(upload): retry transient asset upload failures)

	ic := newTestImmichClient(t, server.URL)
		File:             fshelper.FSName(fstest.MapFS{"video.mp4": {Data: []byte("upload payload")}}, "video.mp4"),
		OriginalFileName: "video.mp4",
		FileSize:         len("upload payload"),
		Checksum:         "checksum",
		FileDate:         time.Unix(0, 0),
	}
	defer la.Close()

	resp, err := ic.uploadAsset(context.Background(), la, EndPointAssetUpload, "")
	if err != nil {
		t.Fatalf("uploadAsset() error = %v", err)
	}
	if resp.Status != UploadCreated {
		t.Fatalf("uploadAsset() status = %q, want %q", resp.Status, UploadCreated)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Fatalf("attempt count = %d, want 3", attempts)
	}
}

func TestShouldRetryUpload(t *testing.T) {
	t.Parallel()

	err := callError{status: http.StatusBadGateway}
<<<<<<< HEAD
	if !shouldRetryUpload(err, 1, 3) {
	if !shouldRetryUpload(err, 1) {
	}
	if shouldRetryUpload(err, 3, 3) {
	if shouldRetryUpload(err, 3) {
	}
	if shouldRetryUpload(context.Canceled, 1, 3) {
=======
	if !shouldRetryUpload(err, 1) {
		t.Fatal("expected 502 to be retryable")
	}
	if shouldRetryUpload(err, 3) {
		t.Fatal("did not expect retry on last attempt")
	}
	if shouldRetryUpload(context.Canceled, 1) {
>>>>>>> d3a63bd (fix(upload): retry transient asset upload failures)
		t.Fatal("did not expect context cancellation to be retryable")
	}
}

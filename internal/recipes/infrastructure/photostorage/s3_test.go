package photostorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// fakeS3 answers the few S3 calls the storage makes, for a bucket holding objects.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string]string // key → content type
	deleted []string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/photos/")
	switch {
	case r.Method == http.MethodHead:
		contentType, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", "2048")
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		prefix := r.URL.Query().Get("prefix")
		var body strings.Builder
		body.WriteString(`<ListBucketResult><IsTruncated>false</IsTruncated>`)
		for key := range f.objects {
			if strings.HasPrefix(key, prefix) {
				body.WriteString("<Contents><Key>" + key + "</Key></Contents>")
			}
		}
		body.WriteString(`</ListBucketResult>`)
		_, _ = w.Write([]byte(body.String()))
	case r.Method == http.MethodPost && r.URL.Query().Has("delete"):
		var body strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			body.Write(buf[:n])
			if err != nil {
				break
			}
		}
		for _, part := range strings.Split(body.String(), "<Key>")[1:] {
			key, _, _ := strings.Cut(part, "</Key>")
			f.deleted = append(f.deleted, key)
			delete(f.objects, key)
		}
		_, _ = w.Write([]byte(`<DeleteResult></DeleteResult>`))
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func newStorage(t *testing.T, objects map[string]string) (*S3, *fakeS3) {
	fake := &fakeS3{objects: objects}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client := s3.New(s3.Options{
		Region:       "eu-west-1",
		Credentials:  credentials.NewStaticCredentialsProvider("key", "secret", ""),
		BaseEndpoint: aws.String(server.URL),
		UsePathStyle: true,
	}, withoutOptionalChecksums)
	return NewS3WithClient(client, "photos"), fake
}

func TestS3(t *testing.T) {
	ctx := context.Background()
	const key = "recipes/owner-1/recipe-1/photo.jpg"

	t.Run("an upload link is a signed PUT with the content type and no checksum to send", func(t *testing.T) {
		storage, _ := newStorage(t, nil)

		upload, err := storage.PresignUpload(ctx, key, "image/jpeg")

		require.NoError(t, err)
		link, err := url.Parse(upload.URL)
		require.NoError(t, err)
		require.Equal(t, "/photos/"+key, link.Path)
		require.Equal(t, "900", link.Query().Get("X-Amz-Expires"))
		require.Contains(t, link.Query().Get("X-Amz-SignedHeaders"), "content-type")
		require.NotContains(t, upload.URL, "checksum")
		require.Equal(t, map[string]string{"Content-Type": "image/jpeg"}, upload.Headers)
	})

	t.Run("finds an uploaded photo, and says when there's none", func(t *testing.T) {
		storage, _ := newStorage(t, map[string]string{key: "image/jpeg"})

		photo, found, err := storage.Stat(ctx, key)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "image/jpeg", photo.ContentType)
		require.Equal(t, int64(2048), photo.Size)

		_, found, err = storage.Stat(ctx, "recipes/owner-1/recipe-1/missing.jpg")
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("deletes every photo under a prefix, and nothing else", func(t *testing.T) {
		storage, fake := newStorage(t, map[string]string{
			"recipes/owner-1/a/1.jpg": "image/jpeg", "recipes/owner-1/b/2.png": "image/png", "recipes/owner-2/c/3.jpg": "image/jpeg",
		})

		require.NoError(t, storage.DeleteAll(ctx, "recipes/owner-1/"))

		require.ElementsMatch(t, []string{"recipes/owner-1/a/1.jpg", "recipes/owner-1/b/2.png"}, fake.deleted)
		require.Contains(t, fake.objects, "recipes/owner-2/c/3.jpg")
	})

	t.Run("without a bucket, uploads are refused and there's nothing to delete", func(t *testing.T) {
		_, err := Disabled{}.PresignUpload(ctx, key, "image/jpeg")
		require.ErrorIs(t, err, apperrors.ErrUnavailable)
		require.NoError(t, Disabled{}.DeleteAll(ctx, "recipes/owner-1/"))
	})
}

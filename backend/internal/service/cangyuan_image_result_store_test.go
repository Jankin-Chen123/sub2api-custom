package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingImageStorage struct {
	keys         []string
	contentTypes []string
	data         [][]byte
	err          error
}

type recordingStreamingImageStorage struct {
	recordingImageStorage
	streamCalls int
	streamed    []byte
}

func (s *recordingStreamingImageStorage) SaveStream(_ context.Context, key, contentType string, body io.Reader, contentLength int64) (string, error) {
	s.streamCalls++
	s.contentTypes = append(s.contentTypes, contentType)
	s.keys = append(s.keys, key)
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	if int64(len(data)) != contentLength {
		return "", io.ErrUnexpectedEOF
	}
	s.streamed = append([]byte(nil), data...)
	return "https://signed.example.test/object", nil
}

func (s *recordingImageStorage) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.keys = append(s.keys, key)
	s.contentTypes = append(s.contentTypes, contentType)
	s.data = append(s.data, append([]byte(nil), data...))
	if s.err != nil {
		return "", s.err
	}
	return "https://signed.example.test/object?private=query", nil
}

func imageResultTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 120, B: 200, A: 255})
		}
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func TestCangyuanImageResultStoreUsesDeterministicObjectReference(t *testing.T) {
	storage := &recordingImageStorage{}
	store := NewCangyuanImageResultStore(storage, "image-results/cangyuan", 0)
	raw := imageResultTestPNG(t, 32, 16)

	refs, actualSize, err := store.Store(context.Background(), "imgjob_abc", []CangyuanImageData{{B64JSON: base64.StdEncoding.EncodeToString(raw)}})
	require.NoError(t, err)
	require.Equal(t, []string{"image-results/cangyuan/imgjob_abc/0.png"}, refs)
	require.Equal(t, "32x16", actualSize)
	require.Equal(t, refs, storage.keys)
	require.Equal(t, []string{"image/png"}, storage.contentTypes)
	require.Equal(t, raw, storage.data[0])
	require.NotContains(t, refs[0], "?")

	refs2, _, err := store.Store(context.Background(), "imgjob_abc", []CangyuanImageData{{B64JSON: base64.StdEncoding.EncodeToString(raw)}})
	require.NoError(t, err)
	require.Equal(t, refs, refs2)
	require.Equal(t, []string{refs[0], refs[0]}, storage.keys)
}

func TestCangyuanImageResultStoreRejectsPrivateAndNonImageOutput(t *testing.T) {
	store := NewCangyuanImageResultStore(&recordingImageStorage{}, "image-results", 0)

	_, _, err := store.Store(context.Background(), "imgjob_private", []CangyuanImageData{{URL: "https://127.0.0.1/private.png"}})
	require.ErrorContains(t, err, "SSRF safety policy")

	_, _, err = store.Store(context.Background(), "imgjob_text", []CangyuanImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("not an image"))}})
	require.ErrorContains(t, err, "not decodable")
}

func TestCangyuanImageResultStoreNeverReturnsSignedStorageURLAsObjectRef(t *testing.T) {
	storage := &recordingImageStorage{}
	store := NewCangyuanImageResultStore(storage, "private", 0)
	raw := imageResultTestPNG(t, 16, 16)

	refs, _, err := store.Store(context.Background(), "imgjob_ref", []CangyuanImageData{{B64JSON: base64.StdEncoding.EncodeToString(raw)}})
	require.NoError(t, err)
	require.Equal(t, "private/imgjob_ref/0.png", refs[0])
	require.NotContains(t, refs[0], "signed.example.test")
}

func TestResolveCodexImageResultDownloadsHTTPSWithoutObjectStorage(t *testing.T) {
	raw := imageResultTestPNG(t, 24, 12)
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(raw)
	}))
	defer upstream.Close()

	storage := &recordingImageStorage{}
	resolver := NewCangyuanImageResultStore(storage, "unused", 0)
	resolver.httpClient = upstream.Client()
	resolver.hostValidator = func(context.Context, string) (bool, error) { return false, nil }
	result, err := resolveCodexImageResultWithStore(context.Background(), CangyuanImageData{
		URL: upstream.URL + "/result.png",
	}, 0, resolver)

	require.NoError(t, err)
	require.Equal(t, "png", result.OutputFormat)
	require.Equal(t, "24x12", result.ActualSize)
	decoded, err := base64.StdEncoding.DecodeString(result.Base64)
	require.NoError(t, err)
	require.Equal(t, raw, decoded)
	require.Empty(t, storage.keys, "Codex URL fallback must never write object storage")
}

func TestCangyuanImageResultStoreUsesBoundedStreamingStorageForLargeOutput(t *testing.T) {
	storage := &recordingStreamingImageStorage{}
	store := NewCangyuanImageResultStore(storage, "private", 64<<20)
	raw := imageResultTestPNG(t, 3840, 2160)

	refs, actualSize, err := store.Store(context.Background(), "imgjob_stream", []CangyuanImageData{{B64JSON: base64.StdEncoding.EncodeToString(raw)}})
	require.NoError(t, err)
	require.Equal(t, []string{"private/imgjob_stream/0.png"}, refs)
	require.Equal(t, "3840x2160", actualSize)
	require.Equal(t, 1, storage.streamCalls)
	require.Equal(t, raw, storage.streamed)
	require.Empty(t, storage.data, "streaming-capable storage must not use the byte-slice Save fallback")
}

func TestCangyuanImageResultStoreStreamsHTTPSURLIntoObjectStorage(t *testing.T) {
	raw := imageResultTestPNG(t, 64, 32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=utf-8")
		_, _ = w.Write(raw)
	}))
	defer server.Close()

	storage := &recordingStreamingImageStorage{}
	store := &CangyuanImageResultStore{
		storage: storage, httpClient: server.Client(), maxBytes: 64 << 20,
		hostValidator: func(context.Context, string) (bool, error) { return false, nil },
		prefix:        "url-results/",
	}

	refs, actualSize, err := store.Store(context.Background(), "imgjob_url", []CangyuanImageData{{URL: server.URL + "/image.png?signature=temporary"}})
	require.NoError(t, err)
	require.Equal(t, []string{"url-results/imgjob_url/0.png"}, refs)
	require.Equal(t, "64x32", actualSize)
	require.Equal(t, raw, storage.streamed)
}

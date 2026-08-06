package service

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCangyuanImageAssetResolverRejectsPrivateHostBeforeDial(t *testing.T) {
	resolver := NewCangyuanImageAssetResolver(cangyuanMaxReferenceImageBytes)
	_, err := resolver.Resolve(context.Background(), "https://127.0.0.1/private.png")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_invalid_reference")
}

func TestCangyuanImageAssetResolverDownloadsAndDeduplicatesByContent(t *testing.T) {
	imageBytes := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()

	allowTestHost := func(context.Context, string) (bool, error) { return false, nil }
	resolver := newCangyuanImageAssetResolver(server.Client(), cangyuanMaxReferenceImageBytes, allowTestHost)
	assets, err := resolver.ResolveUnique(context.Background(), []string{server.URL + "/one", server.URL + "/two"})
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Equal(t, "image/png", assets[0].ContentType)
	require.Equal(t, 32, assets[0].Width)
	require.Len(t, assets[0].SHA256, 64)
}

func TestCangyuanImageAssetResolverRejectsMIMEConfusionAndOversize(t *testing.T) {
	imageBytes := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mime" {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(imageBytes)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()

	allowTestHost := func(context.Context, string) (bool, error) { return false, nil }
	resolver := newCangyuanImageAssetResolver(server.Client(), int64(len(imageBytes)-1), allowTestHost)
	_, err := resolver.Resolve(context.Background(), server.URL+"/oversize")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_invalid_reference")

	resolver = newCangyuanImageAssetResolver(server.Client(), cangyuanMaxReferenceImageBytes, allowTestHost)
	_, err = resolver.Resolve(context.Background(), server.URL+"/mime")
	require.Error(t, err)
	require.Contains(t, err.Error(), "MIME type")
}

func TestCangyuanImageAssetResolverRejectsHTTPSRedirectToHTTP(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	allowTestHost := func(context.Context, string) (bool, error) { return false, nil }
	resolver := newCangyuanImageAssetResolver(source.Client(), cangyuanMaxReferenceImageBytes, allowTestHost)
	_, err := resolver.Resolve(context.Background(), source.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_reference_download_failed")
}

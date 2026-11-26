package upgrade_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relayx-tech/panel/internal/upgrade"
)

func TestManifestFetchAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"version":"1.4.0","channel":"stable","assets":[{"name":"panel-linux-amd64","url":"https://example.com/panel","sha256":"abc","size":10,"arch":"amd64"}]}`)
	sig := ed25519.Sign(priv, manifest)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_, _ = w.Write(manifest)
		case "/manifest.json.sig":
			_, _ = w.Write([]byte(hex.EncodeToString(sig)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := &upgrade.HTTPManifestSource{
		ManifestURL: server.URL + "/manifest.json",
		PublicKeys:  []string{hex.EncodeToString(pub)},
	}
	release, err := source.Fetch(context.Background(), "stable")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if release.Version != "1.4.0" || len(release.Assets) != 1 {
		t.Fatalf("unexpected release: %+v", release)
	}

	// 篡改 manifest 后签名必须失败。
	if err := upgrade.VerifyManifest(append(manifest, ' '), hex.EncodeToString(sig), []string{hex.EncodeToString(pub)}); err == nil {
		t.Fatal("tampered manifest should fail verification")
	}
}

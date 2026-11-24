package upgrade

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ManifestFile 是 manifest.json 的结构（design.md §5.6）。
type ManifestFile struct {
	Version            string    `json:"version"`
	Channel            string    `json:"channel"`
	Assets             []Asset   `json:"assets"`
	MinPanelVersion    string    `json:"minPanelVersion"`
	MinContractVersion string    `json:"minContractVersion"`
	PublishedAt        string    `json:"publishedAt"`
	NotAfter           time.Time `json:"-"`
}

// HTTPManifestSource 从 HTTPS 更新源拉取并验签 manifest。
type HTTPManifestSource struct {
	ManifestURL string
	PublicKeys  []string // hex 编码的 ed25519 公钥
	HTTP        *http.Client
}

var _ ManifestSource = (*HTTPManifestSource)(nil)

func (s *HTTPManifestSource) Fetch(ctx context.Context, channel string) (*ReleaseManifest, error) {
	if s.ManifestURL == "" {
		return nil, fmt.Errorf("update source is not configured")
	}
	httpClient := s.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	manifestBytes, err := s.get(ctx, httpClient, s.ManifestURL)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	sigBytes, err := s.get(ctx, httpClient, s.ManifestURL+".sig")
	if err != nil {
		return nil, fmt.Errorf("fetch manifest signature: %w", err)
	}
	if err := VerifyManifest(manifestBytes, strings.TrimSpace(string(sigBytes)), s.PublicKeys); err != nil {
		return nil, err
	}
	var file ManifestFile
	if err := json.Unmarshal(manifestBytes, &file); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if file.Version == "" {
		return nil, fmt.Errorf("manifest version is empty")
	}
	return &ReleaseManifest{
		Version: file.Version, Channel: file.Channel, Assets: file.Assets,
		ManifestURL: s.ManifestURL, Signature: strings.TrimSpace(string(sigBytes)),
	}, nil
}

func (s *HTTPManifestSource) get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// VerifyManifest 校验 ed25519 签名；任一公钥通过即视为有效。
func VerifyManifest(manifest []byte, signatureHex string, publicKeysHex []string) error {
	if len(publicKeysHex) == 0 {
		return fmt.Errorf("no update public keys configured")
	}
	sig, err := hex.DecodeString(strings.TrimSpace(signatureHex))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid manifest signature encoding")
	}
	for _, keyHex := range publicKeysHex {
		key, err := hex.DecodeString(strings.TrimSpace(keyHex))
		if err != nil || len(key) != ed25519.PublicKeySize {
			continue
		}
		if ed25519.Verify(ed25519.PublicKey(key), manifest, sig) {
			return nil
		}
	}
	return fmt.Errorf("manifest signature verification failed")
}

// Package cryptobox 提供 AES-256-GCM 加密，密文自带 key ID 前缀以支持密钥轮换。
// 用于 Panel 的配置版本 / 快照 / 隧道材料加密（design §4.9、§6.4）。
package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Box 持有单一 key 的 AEAD 并标注 key ID。
type Box struct {
	keyID string
	aead  cipher.AEAD
}

// New 用 32 字节主密钥构造；keyID 为空时默认 k1。
func New(masterKey []byte, keyID string) (*Box, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key 必须 32 字节，实际 %d", len(masterKey))
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if keyID == "" {
		keyID = "k1"
	}
	if len(keyID) > 255 {
		return nil, errors.New("key ID 过长")
	}
	return &Box{keyID: keyID, aead: aead}, nil
}

// KeyID 返回当前 key ID。
func (b *Box) KeyID() string { return b.keyID }

// Seal 输出格式：keyIDLen(1) | keyID | nonce(12) | ciphertext。
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := b.aead.Seal(nil, nonce, plaintext, []byte(b.keyID))
	out := make([]byte, 0, 1+len(b.keyID)+len(nonce)+len(ct))
	out = append(out, byte(len(b.keyID)))
	out = append(out, b.keyID...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Open 解析并解密 Seal 的输出；key ID 不匹配或密文被篡改返回错误。
func (b *Box) Open(sealed []byte) ([]byte, error) {
	if len(sealed) < 1 {
		return nil, errors.New("密文过短")
	}
	idLen := int(sealed[0])
	if len(sealed) < 1+idLen+b.aead.NonceSize() {
		return nil, errors.New("密文过短")
	}
	keyID := string(sealed[1 : 1+idLen])
	if keyID != b.keyID {
		return nil, fmt.Errorf("key ID 不匹配: %s", keyID)
	}
	rest := sealed[1+idLen:]
	nonce := rest[:b.aead.NonceSize()]
	ct := rest[b.aead.NonceSize():]
	return b.aead.Open(nil, nonce, ct, []byte(keyID))
}

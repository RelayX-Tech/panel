package cryptobox_test

import (
	"bytes"
	"testing"

	"github.com/relayx-tech/panel/internal/cryptobox"
)

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	box, err := cryptobox.New(testKey(), "k1")
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte(`{"inbounds":[]}`)
	sealed, err := box.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(sealed, plaintext) {
		t.Fatal("密文不应等于明文")
	}
	got, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("解密结果 = %q", got)
	}
}

func TestOpenTampered(t *testing.T) {
	box, _ := cryptobox.New(testKey(), "k1")
	sealed, _ := box.Seal([]byte("secret"))
	sealed[len(sealed)-1] ^= 0xff
	if _, err := box.Open(sealed); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}

func TestOpenKeyMismatch(t *testing.T) {
	box1, _ := cryptobox.New(testKey(), "k1")
	box2, _ := cryptobox.New(testKey(), "k2")
	sealed, _ := box1.Seal([]byte("secret"))
	if _, err := box2.Open(sealed); err == nil {
		t.Fatal("key ID 不匹配应失败")
	}
}

func TestInvalidKeyLength(t *testing.T) {
	if _, err := cryptobox.New([]byte("short"), "k1"); err == nil {
		t.Fatal("非 32 字节主密钥应失败")
	}
}

package aes7z

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sync"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type cacheKey struct {
	password string
	cycles   int
	salt     string // []byte isn't comparable
}

const cacheSize = 10

//nolint:gochecknoglobals
var (
	keyCacheMu sync.Mutex
	keyCache   = make(map[cacheKey][]byte, cacheSize)
	keyOrder   = make([]cacheKey, 0, cacheSize)
)

func calculateKey(password string, cycles int, salt []byte) ([]byte, error) {
	ck := cacheKey{
		password: password,
		cycles:   cycles,
		salt:     hex.EncodeToString(salt),
	}

	keyCacheMu.Lock()
	if key, ok := keyCache[ck]; ok {
		keyCacheMu.Unlock()
		return key, nil
	}
	keyCacheMu.Unlock()

	b := bytes.NewBuffer(salt)

	// Convert password to UTF-16LE
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	t := transform.NewWriter(b, utf16le.NewEncoder())
	_, _ = t.Write([]byte(password))

	key := make([]byte, sha256.Size)
	if cycles == 0x3f {
		copy(key, b.Bytes())
	} else {
		h := sha256.New()
		for i := uint64(0); i < 1<<cycles; i++ {
			// These will never error
			_, _ = h.Write(b.Bytes())
			_ = binary.Write(h, binary.LittleEndian, i)
		}

		copy(key, h.Sum(nil))
	}

	keyCacheMu.Lock()
	if _, ok := keyCache[ck]; !ok {
		if len(keyOrder) >= cacheSize {
			oldest := keyOrder[0]
			keyOrder = keyOrder[1:]
			delete(keyCache, oldest)
		}
		keyOrder = append(keyOrder, ck)
		keyCache[ck] = key
	}
	keyCacheMu.Unlock()

	return key, nil
}

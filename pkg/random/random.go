package random

import (
	"crypto/rand"
	"encoding/json"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // 52 possibilities
	letterIdxBits = 6                                                      // 6 bits to represent 64 possibilities / indexes
	letterIdxMask = 1<<letterIdxBits - 1                                   // All 1-bits, as many as letterIdxBits
)

// AlphaString returns a random alphanumeric string of the requested length
// https://stackoverflow.com/a/35615565/145125
func AlphaString(length int) string {
	result := make([]byte, length)
	bufferSize := int(float64(length) * 1.3)
	for i, j, randomBytes := 0, 0, []byte{}; i < length; j++ {
		if j%bufferSize == 0 {
			randomBytes = secureRandomBytes(bufferSize)
		}
		if idx := int(randomBytes[j%length] & letterIdxMask); idx < len(letterBytes) {
			result[i] = letterBytes[idx]
			i++
		}
	}

	return string(result)
}

// secureRandomBytes returns the requested number of bytes using crypto/rand
func secureRandomBytes(length int) []byte {
	randomBytes := make([]byte, length)
	_, _ = rand.Read(randomBytes)
	return randomBytes
}

// CryptoString returns a random numeric string of the requested length
func CryptoString(bits int) (string, error) {
	RandomCrypto, randErr := rand.Prime(rand.Reader, bits)
	if randErr != nil {
		return "", randErr
	}
	data, marshalErr := json.Marshal(RandomCrypto)
	if marshalErr != nil {
		return "", marshalErr
	}
	return string(data), nil
}

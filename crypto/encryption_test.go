package crypto_test

import (
	"testing"

	"github.com/henrywhitaker3/windowframe/crypto"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

func TestItGeneratesAKey(t *testing.T) {
	type test struct {
		name        string
		size        int
		shouldError bool
	}

	tcs := []test{
		{
			name:        "generates 128 bit key",
			size:        128,
			shouldError: false,
		},
		{
			name:        "generates 192 bit key",
			size:        192,
			shouldError: false,
		},
		{
			name:        "generates 256 bit key",
			size:        256,
			shouldError: false,
		},
		{
			name:        "errors for 512 bit key",
			size:        512,
			shouldError: true,
		},
		{
			name:        "fails with invalid key length",
			size:        9,
			shouldError: true,
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			_, err := crypto.GenerateAesKey(c.size)
			if c.shouldError {
				require.NotNil(t, err)
			}
		})
	}
}

func TestItEncrpytsAndDecryptsVLittleData(t *testing.T) {
	input := test.Word()
	encrypt(t, input)
}

func encrypt(t *testing.T, input string) {
	key, err := crypto.GenerateAesKey(256)
	require.Nil(t, err)
	e, err := crypto.NewEncryptor(key)
	require.Nil(t, err)
	cipher, err := e.Encrypt([]byte(input))
	require.Nil(t, err)
	require.NotEqual(t, cipher, []byte(input))

	decrypted, err := e.Decrypt(cipher)
	require.Nil(t, err)
	require.Equal(t, input, string(decrypted))
}

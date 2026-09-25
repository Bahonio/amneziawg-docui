// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

package awg

import (
	"crypto/rand"
	"encoding/base64"
)

type KeyPair struct{ Private, Public string }

// GenerateKeyPairE never falls back in production: missing host tools are
// reported to the UI and cannot select another VPN implementation.
func (t *Tools) GenerateKeyPairE() (KeyPair, error) {
	return t.backend.GenerateKeyPair()
}

func (t *Tools) GenerateKeyPair() KeyPair { keys, _ := t.GenerateKeyPairE(); return keys }

func (t *Tools) GeneratePresharedKeyE() (string, error) {
	return t.backend.GeneratePresharedKey()
}

func (t *Tools) GeneratePresharedKey() string { key, _ := t.GeneratePresharedKeyE(); return key }

func RandomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(b)
}

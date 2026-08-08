package scripthash

import (
	"crypto/sha256"
	"encoding/hex"
)

func AddressToScripthash(address string) (string, error) {
	hash := sha256.Sum256([]byte(address))
	return hex.EncodeToString(hash[:]), nil
}

func ScriptToScripthash(script string) (string, error) {
	scriptBytes, err := hex.DecodeString(script)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(scriptBytes)
	return hex.EncodeToString(hash[:]), nil
}
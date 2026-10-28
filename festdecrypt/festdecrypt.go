package festdecrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func ParseJsonFromDecryptedBlob(blob string) string {
	startIdx := strings.Index(blob, "{")
	endIdx := strings.LastIndex(blob, "}")

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return blob
	}

	return blob[startIdx : endIdx+1]
}

func decryptEnvelope(EVString string, Bearer string) (string, error) {
	envelope, err := base64.StdEncoding.DecodeString(EVString)
	if err != nil {
		return "", fmt.Errorf("failed to decode envelope: %s", err)
	}

	if len(envelope) < 5 {
		return "", errors.New("envelope is too short")
	}

	if envelope[0] != 1 {
		return "", errors.New("envelope header is invalid")
	}

	fourthbyteofEnv := int(envelope[3])
	fifthbyteofEnv := int(envelope[4])

	envelope = envelope[5:]

	if fourthbyteofEnv > 0x10 {
		return "", errors.New("invalid bearer subkey length")
	}

	if len(Bearer) < fourthbyteofEnv {
		return "", errors.New("bearer is shorter than the envelope subkey length")
	}

	subkey := []byte(Bearer[len(Bearer)-fourthbyteofEnv:])
	if len(subkey) != fourthbyteofEnv {
		return "", errors.New("invalid bearer subkey length")
	}

	tailLen := 0x10 - fourthbyteofEnv
	if len(envelope) < tailLen {
		return "", errors.New("envelope is too short for the key material")
	}

	finalkey := make([]byte, 0, 0x10)
	finalkey = append(finalkey, envelope[len(envelope)-tailLen:]...)
	finalkey = append(finalkey, subkey...)

	startIndex := fourthbyteofEnv + len(envelope) - 0x10
	endIndex := startIndex + tailLen
	if startIndex < 0 || endIndex > len(envelope) {
		return "", errors.New("invalid envelope offsets")
	}

	envelope = append(envelope[:startIndex], envelope[endIndex:]...)

	if fifthbyteofEnv > len(envelope) {
		return "", errors.New("invalid envelope payload offset")
	}

	encryptedIntArray := envelope[fifthbyteofEnv:]
	if len(encryptedIntArray)%aes.BlockSize != 0 {
		return "", fmt.Errorf("encrypted payload length %d is not a multiple of the block size", len(encryptedIntArray))
	}

	block, err := aes.NewCipher(finalkey)
	if err != nil {
		return "", err
	}

	IV := make([]byte, aes.BlockSize)
	cipher.NewCBCDecrypter(block, IV).CryptBlocks(encryptedIntArray, encryptedIntArray)

	return ParseJsonFromDecryptedBlob(string(encryptedIntArray)), nil
}

type CDMJson struct {
	Keys []struct {
		K   string `json:"k"`
		Kid string `json:"kid"`
		Kty string `json:"kty"`
	} `json:"keys"`
}

func addBase64Padding(b64blob []byte) string {
	paddingNeeded := (4 - len(b64blob)%4) % 4

	for i := 0; i < paddingNeeded; i++ {
		b64blob = append(b64blob, '=')
	}

	return string(b64blob)
}

func GetFestEncryptionKey(EVString string, Bearer string) (string, error) {
	var cdmobj CDMJson

	strevobj, err := decryptEnvelope(EVString, Bearer)

	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(strevobj), &cdmobj)

	if err != nil {
		return "", err
	}

	if len(cdmobj.Keys) == 0 {
		return "", errors.New("no keys found in ev blob")
	}

	key := cdmobj.Keys[0].K

	key = strings.ReplaceAll(key, "-", "+")
	key = strings.ReplaceAll(key, "_", "/")

	decodedKey, err := base64.StdEncoding.DecodeString(addBase64Padding([]byte(key)))
	if err != nil {
		return "", fmt.Errorf("failed to decode key %v", err)
	}

	key = hex.EncodeToString(decodedKey)

	return key, nil
}

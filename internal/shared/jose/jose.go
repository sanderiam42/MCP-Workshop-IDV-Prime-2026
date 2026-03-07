package jose

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var rawBase64URL = base64.RawURLEncoding

type KeySet struct {
	PrivateKey *rsa.PrivateKey
	KeyID      string
}

func LoadOrCreateRSAKey(path string) (*KeySet, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}

		block := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		}

		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &KeySet{
		PrivateKey: privateKey,
		KeyID:      deriveKeyID(&privateKey.PublicKey),
	}, nil
}

func (k *KeySet) SignToken(claims map[string]any, typ string) (string, error) {
	if k == nil || k.PrivateKey == nil {
		return "", errors.New("private key is required")
	}

	header := map[string]any{
		"alg": "RS256",
		"kid": k.KeyID,
	}
	if typ != "" {
		header["typ"] = typ
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := rawBase64URL.EncodeToString(headerJSON) + "." + rawBase64URL.EncodeToString(payloadJSON)
	sum := sha256.Sum256([]byte(signingInput))

	signature, err := rsa.SignPKCS1v15(rand.Reader, k.PrivateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + rawBase64URL.EncodeToString(signature), nil
}

func VerifyToken(token string, publicKey *rsa.PublicKey) (map[string]any, map[string]any, error) {
	if publicKey == nil {
		return nil, nil, errors.New("public key is required")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("token must have three parts")
	}

	headerBytes, err := rawBase64URL.DecodeString(parts[0])
	if err != nil {
		return nil, nil, err
	}

	payloadBytes, err := rawBase64URL.DecodeString(parts[1])
	if err != nil {
		return nil, nil, err
	}

	signature, err := rawBase64URL.DecodeString(parts[2])
	if err != nil {
		return nil, nil, err
	}

	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, err
	}

	if HeaderString(header, "alg") != "RS256" {
		return nil, nil, fmt.Errorf("unsupported alg %q", HeaderString(header, "alg"))
	}

	signingInput := parts[0] + "." + parts[1]
	sum := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return nil, nil, err
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, nil, err
	}

	return header, claims, nil
}

func DecodeWithoutVerify(token string) (map[string]any, map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("token must have three parts")
	}

	headerBytes, err := rawBase64URL.DecodeString(parts[0])
	if err != nil {
		return nil, nil, err
	}
	payloadBytes, err := rawBase64URL.DecodeString(parts[1])
	if err != nil {
		return nil, nil, err
	}

	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, err
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, nil, err
	}

	return header, claims, nil
}

func ValidateTimeClaims(claims map[string]any, now time.Time) error {
	exp := ClaimInt64(claims, "exp")
	if exp == 0 {
		return errors.New("missing exp claim")
	}
	if now.Unix() >= exp {
		return errors.New("token expired")
	}

	iat := ClaimInt64(claims, "iat")
	if iat == 0 {
		return errors.New("missing iat claim")
	}
	if iat > now.Add(2*time.Minute).Unix() {
		return errors.New("token issued in the future")
	}

	return nil
}

func (k *KeySet) JWKS() map[string]any {
	return map[string]any{
		"keys": []any{k.JWK()},
	}
}

func (k *KeySet) JWK() map[string]any {
	publicKey := &k.PrivateKey.PublicKey
	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": k.KeyID,
		"n":   rawBase64URL.EncodeToString(publicKey.N.Bytes()),
		"e":   rawBase64URL.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}
}

func PublicKeyFromJWKS(data []byte) (*rsa.PublicKey, string, error) {
	var payload struct {
		Keys []struct {
			KeyID string `json:"kid"`
			KTY   string `json:"kty"`
			N     string `json:"n"`
			E     string `json:"e"`
		} `json:"keys"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, "", err
	}
	if len(payload.Keys) == 0 {
		return nil, "", errors.New("jwks is empty")
	}

	key := payload.Keys[0]
	if key.KTY != "RSA" {
		return nil, "", fmt.Errorf("unsupported jwk type %q", key.KTY)
	}

	nBytes, err := rawBase64URL.DecodeString(key.N)
	if err != nil {
		return nil, "", err
	}
	eBytes, err := rawBase64URL.DecodeString(key.E)
	if err != nil {
		return nil, "", err
	}

	publicKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}

	return publicKey, key.KeyID, nil
}

func ClaimString(claims map[string]any, name string) string {
	value, ok := claims[name]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func HeaderString(header map[string]any, name string) string {
	value, ok := header[name]
	if !ok {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func ClaimInt64(claims map[string]any, name string) int64 {
	value, ok := claims[name]
	if !ok {
		return 0
	}

	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		number, _ := typed.Int64()
		return number
	default:
		return 0
	}
}

func TokenPreview(token string) string {
	if len(token) <= 24 {
		return token
	}
	return token[:16] + "..." + token[len(token)-8:]
}

func deriveKeyID(publicKey *rsa.PublicKey) string {
	sum := sha256.Sum256(x509.MarshalPKCS1PublicKey(publicKey))
	return rawBase64URL.EncodeToString(sum[:8])
}

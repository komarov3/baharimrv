package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"

	"github.com/golang-jwt/jwt/v5"
)

type Signer struct {
	private crypto.PrivateKey
	public  crypto.PublicKey
	kid     string
	method  jwt.SigningMethod
}

func NewSigner(privatePEM, kid string) (*Signer, error) {
	var key crypto.PrivateKey
	if privatePEM == "" {
		generated, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral OIDC signing key: %w", err)
		}
		key = generated
	} else {
		block, _ := pem.Decode([]byte(privatePEM))
		if block == nil {
			return nil, fmt.Errorf("OIDC signing key must be PEM encoded")
		}
		if parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes); parseErr == nil {
			key = parsed
		} else if parsedRSA, rsaErr := x509.ParsePKCS1PrivateKey(block.Bytes); rsaErr == nil {
			key = parsedRSA
		} else if parsedEC, ecErr := x509.ParseECPrivateKey(block.Bytes); ecErr == nil {
			key = parsedEC
		} else {
			return nil, fmt.Errorf("unsupported OIDC signing key")
		}
	}
	signer := &Signer{private: key, kid: kid}
	switch typed := key.(type) {
	case *rsa.PrivateKey:
		signer.public = &typed.PublicKey
		signer.method = jwt.SigningMethodRS256
	case *ecdsa.PrivateKey:
		signer.public = &typed.PublicKey
		signer.method = jwt.SigningMethodES256
	default:
		return nil, fmt.Errorf("OIDC signing key must be RSA or ECDSA")
	}
	return signer, nil
}

func (s *Signer) Sign(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(s.method, claims)
	token.Header["kid"] = s.kid
	token.Header["typ"] = "JWT"
	return token.SignedString(s.private)
}

func (s *Signer) Parse(token string) (jwt.MapClaims, error) {
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if token.Method != s.method || token.Header["kid"] != s.kid {
			return nil, fmt.Errorf("unexpected token algorithm or kid")
		}
		return s.public, nil
	}, jwt.WithValidMethods([]string{s.method.Alg()}))
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func (s *Signer) JWKS() map[string]any {
	key := map[string]any{"kid": s.kid, "use": "sig"}
	switch public := s.public.(type) {
	case *rsa.PublicKey:
		key["kty"] = "RSA"
		key["alg"] = "RS256"
		key["n"] = base64.RawURLEncoding.EncodeToString(public.N.Bytes())
		key["e"] = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(public.E)).Bytes())
	case *ecdsa.PublicKey:
		key["kty"] = "EC"
		key["alg"] = "ES256"
		key["crv"] = public.Curve.Params().Name
		size := (public.Curve.Params().BitSize + 7) / 8
		key["x"] = base64.RawURLEncoding.EncodeToString(public.X.FillBytes(make([]byte, size)))
		key["y"] = base64.RawURLEncoding.EncodeToString(public.Y.FillBytes(make([]byte, size)))
	}
	return map[string]any{"keys": []any{key}}
}

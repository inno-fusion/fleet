package apple_mdm

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"time"

	"github.com/micromdm/micromdm/pkg/crypto/mdmcertutil"
)

// SignAPNSCSRLocally uses the operator's Apple-issued vendor identity, never a hosted signer.
func SignAPNSCSRLocally(csr *x509.CertificateRequest, certPath, keyPath, chainPath string) ([]byte, error) {
	if certPath == "" || keyPath == "" || chainPath == "" {
		return nil, errors.New("local MDM vendor signing requires certificate, private key and Apple chain paths")
	}
	if csr == nil {
		return nil, errors.New("missing APNs certificate signing request")
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, errors.New("invalid APNs certificate signing request")
	}
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, errors.New("load matching MDM vendor certificate and private key: " + err.Error())
	}
	key, ok := pair.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("MDM vendor signing requires an RSA private key")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, err
	}
	chainPEM, err := os.ReadFile(chainPath)
	if err != nil {
		return nil, err
	}
	var chain []*x509.Certificate
	for remaining := bytes.TrimSpace(chainPEM); len(remaining) > 0; {
		block, rest := pem.Decode(remaining)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, errors.New("MDM vendor chain must contain only PEM certificates")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		chain = append(chain, cert)
		remaining = bytes.TrimSpace(rest)
	}
	if len(chain) == 0 {
		return nil, errors.New("MDM vendor chain is empty")
	}
	previous := leaf
	for _, cert := range chain {
		if time.Now().Before(previous.NotBefore) || time.Now().After(previous.NotAfter) {
			return nil, errors.New("MDM vendor certificate chain is expired or not yet valid")
		}
		if err := previous.CheckSignatureFrom(cert); err != nil {
			return nil, errors.New("invalid MDM vendor certificate chain: " + err.Error())
		}
		previous = cert
	}
	if time.Now().Before(previous.NotBefore) || time.Now().After(previous.NotAfter) {
		return nil, errors.New("MDM vendor root certificate is expired or not yet valid")
	}
	if err := previous.CheckSignatureFrom(previous); err != nil {
		return nil, errors.New("MDM vendor chain must end with a self-signed root")
	}
	digest := sha256.Sum256(csr.Raw)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return nil, err
	}
	request := mdmcertutil.PushCertificateRequest{
		PushCertRequestCSR:       base64.StdEncoding.EncodeToString(csr.Raw),
		PushCertSignature:        base64.StdEncoding.EncodeToString(signature),
		PushCertCertificateChain: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})) + string(chainPEM),
	}
	return request.Encode()
}

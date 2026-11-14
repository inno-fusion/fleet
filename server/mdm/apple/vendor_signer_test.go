package apple_mdm

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/micromdm/micromdm/pkg/crypto/mdmcertutil"
	"github.com/micromdm/plist"
	"github.com/stretchr/testify/require"
)

func TestLocalVendorCSRSigning(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test root"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	vendorTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "MDM Vendor: Test"}, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	vendorDER, err := x509.CreateCertificate(rand.Reader, vendorTemplate, ca, &key.PublicKey, key)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath, chainPath := filepath.Join(dir, "vendor.pem"), filepath.Join(dir, "vendor.key"), filepath.Join(dir, "chain.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: vendorDER}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	require.NoError(t, os.WriteFile(chainPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	csr, _, err := GenerateAPNSCSRKey("admin@example.test", "Test")
	require.NoError(t, err)
	encoded, err := SignAPNSCSRLocally(csr, certPath, keyPath, chainPath)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	require.NoError(t, err)
	var request mdmcertutil.PushCertificateRequest
	require.NoError(t, plist.Unmarshal(decoded, &request))
	require.Equal(t, base64.StdEncoding.EncodeToString(csr.Raw), request.PushCertRequestCSR)
	signature, err := base64.StdEncoding.DecodeString(request.PushCertSignature)
	require.NoError(t, err)
	digest := sha256.Sum256(csr.Raw)
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature))
	require.Contains(t, request.PushCertCertificateChain, "BEGIN CERTIFICATE")

	_, err = SignAPNSCSRLocally(csr, certPath, "", chainPath)
	require.Error(t, err)
	_, err = SignAPNSCSRLocally(nil, certPath, keyPath, chainPath)
	require.ErrorContains(t, err, "missing APNs")
	tampered := *csr
	tampered.Signature = append([]byte(nil), csr.Signature...)
	tampered.Signature[0] ^= 0xff
	_, err = SignAPNSCSRLocally(&tampered, certPath, keyPath, chainPath)
	require.ErrorContains(t, err, "invalid APNs")

	chain := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	require.NoError(t, os.WriteFile(chainPath, append(chain, []byte("\n 	\n")...), 0600))
	_, err = SignAPNSCSRLocally(csr, certPath, keyPath, chainPath)
	require.NoError(t, err, "PEM files may have trailing whitespace")
	for _, malformed := range [][]byte{nil, []byte("not a chain"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: caDER})} {
		require.NoError(t, os.WriteFile(chainPath, malformed, 0600))
		_, err = SignAPNSCSRLocally(csr, certPath, keyPath, chainPath)
		require.Error(t, err)
	}
	require.NoError(t, os.WriteFile(chainPath, chain, 0600))

	expired := *vendorTemplate
	expired.NotBefore = time.Now().Add(-2 * time.Hour)
	expired.NotAfter = time.Now().Add(-time.Hour)
	expiredDER, err := x509.CreateCertificate(rand.Reader, &expired, ca, &key.PublicKey, key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: expiredDER}), 0600))
	_, err = SignAPNSCSRLocally(csr, certPath, keyPath, chainPath)
	require.ErrorContains(t, err, "expired")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: vendorDER}), 0600))

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(other)}), 0600))
	_, err = SignAPNSCSRLocally(csr, certPath, keyPath, chainPath)
	require.ErrorContains(t, err, "match")
}

func TestCheckSelfSignedRootAppleSHA1Root(t *testing.T) {
	appleRoot := readTestCertPEM(t, "testdata/apple-root-ca.pem")
	require.Equal(t, x509.SHA1WithRSA, appleRoot.SignatureAlgorithm)
	require.NoError(t, checkSelfSignedRoot(appleRoot))

	tampered := *appleRoot
	tampered.Signature = append([]byte(nil), appleRoot.Signature...)
	tampered.Signature[0] ^= 0xff
	require.Error(t, checkSelfSignedRoot(&tampered))

	wwdr := readTestCertPEM(t, "testdata/apple-wwdr-ca-g3.pem")
	require.NoError(t, wwdr.CheckSignatureFrom(appleRoot))
	require.Error(t, checkSelfSignedRoot(wwdr))
}

func readTestCertPEM(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	block, _ := pem.Decode(data)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}

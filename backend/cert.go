// cert.go — Self-signed certificate utility code for future HTTPS/TLS support.
//
// NOTE: This file is NOT currently wired into the application. The server in
// main.go uses plain http.ListenAndServe (no TLS). The USE_HTTPS flag in
// .env.example is aspirational. These functions (generateSelfSignedCert,
// resolveCertPath) are kept as ready-made utilities for when HTTPS support
// is implemented.
//
// Do NOT remove — the logic is correct and will be needed.

package main

import (
        "crypto/rand"
        "crypto/rsa"
        "crypto/x509"
        "crypto/x509/pkix"
        "encoding/pem"
        "math/big"
        "net"
        "os"
        "path/filepath"
        "time"
)

// generateSelfSignedCert creates a self-signed certificate for localhost development
func generateSelfSignedCert(certFile, keyFile string) error {
        // Ensure directory exists
        dir := filepath.Dir(certFile)
        if err := os.MkdirAll(dir, 0o700); err != nil {
                return err
        }

        // Generate private key
        privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
        if err != nil {
                return err
        }

        // Create certificate template
        template := x509.Certificate{
                SerialNumber: big.NewInt(1),
                Subject: pkix.Name{
                        Organization: []string{"Entropy GUI"},
                        CommonName:   "127.0.0.1",
                },
                NotBefore:             time.Now(),
                NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
                KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
                ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
                BasicConstraintsValid: true,
                DNSNames:              []string{"localhost", "127.0.0.1"},
                IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
        }

        // Self-sign the certificate
        certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
        if err != nil {
                return err
        }

        // Write certificate
        certOut, err := os.Create(certFile)
        if err != nil {
                return err
        }
        defer certOut.Close()

        if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
                return err
        }

        // Write private key
        keyOut, err := os.Create(keyFile)
        if err != nil {
                return err
        }
        defer keyOut.Close()
        os.Chmod(keyFile, 0o600) // Restrict key file permissions

        keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
        if err != nil {
                return err
        }

        return pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
}

// resolveCertPath finds the best place to store certificates
func resolveCertPath(filename string) string {
        // Try beside the binary first
        if bin, err := os.Executable(); err == nil {
                dir := filepath.Dir(bin)
                certPath := filepath.Join(dir, filename)
                if _, err := os.Stat(dir); err == nil {
                        return certPath
                }
        }

        // Fallback to user config directory
        if home, err := os.UserHomeDir(); err == nil {
                configDir := filepath.Join(home, ".config", "entropy-gui")
                return filepath.Join(configDir, filename)
        }

        // Last resort: current directory
        return filename
}



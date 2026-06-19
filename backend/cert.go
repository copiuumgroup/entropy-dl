// cert.go — Self-signed certificate utility for HTTPS/TLS support.
//
// When USE_HTTPS=1, the server generates (once) and reuses a self-signed
// certificate covering localhost, 127.0.0.1, ::1, and every non-loopback IPv4
// the host advertises on its interfaces. That last part matters for the
// homelab case: a household device connecting to the server by its LAN IP
// (e.g. https://192.168.1.10:8001) would otherwise hit a name-mismatch
// warning because the cert only names "localhost". Covering the LAN IPs up
// front removes that friction on first connect.
//
// The cert is self-signed, so browsers still warn the first time per device
// until the user (or an mkcert CA) trusts it. See README "Homelab" section
// for the recommended upgrade paths (mkcert, Caddy, Tailscale).

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// generateSelfSignedCert creates a self-signed certificate covering localhost,
// both loopback IPs, and every LAN IPv4 the host exposes. See file header.
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

	// Collect every IP this cert should cover.
	dnsNames := []string{"localhost"}
	ipSet := map[string]net.IP{
		"127.0.0.1": net.IPv4(127, 0, 0, 1),
		"::1":       net.IPv6loopback,
	}
	for _, ip := range lanIPv4Addrs() {
		ipSet[ip.String()] = ip
	}
	ips := make([]net.IP, 0, len(ipSet))
	for _, ip := range ipSet {
		ips = append(ips, ip)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Entropy GUI"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
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

// ensureCerts returns paths to a cert and key, generating them on first use.
// If both files already exist they are reused as-is (the 10-year validity makes
// rotation a non-issue for a self-signed homelab cert).
func ensureCerts() (certFile, keyFile string, err error) {
	certFile = resolveCertPath("entropy.crt")
	keyFile = resolveCertPath("entropy.key")
	if _, e := os.Stat(certFile); e == nil {
		if _, e := os.Stat(keyFile); e == nil {
			return certFile, keyFile, nil // already provisioned
		}
	}
	if gErr := generateSelfSignedCert(certFile, keyFile); gErr != nil {
		return "", "", fmt.Errorf("generate self-signed cert: %w", gErr)
	}
	logCertPaths(certFile, keyFile)
	return certFile, keyFile, nil
}

// lanIPv4Addrs returns the host's non-loopback, non-link-local, globally-routable
// (or LAN) IPv4 addresses. Used to populate the cert so devices connecting by
// LAN IP don't get a name-mismatch warning. Best-effort: failures return nil.
func lanIPv4Addrs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, iface := range ifaces {
		// Skip down interfaces and loopback interfaces (handled explicitly elsewhere).
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil {
				continue
			}
			ip4 := ip.To4()
			if ip4.IsLoopback() || ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast() {
				continue
			}
			out = append(out, ip4)
		}
	}
	return out
}

// logCertPaths prints where the cert landed. Kept as a function so the
// "first run" vs "reused" distinction lives next to generation.
func logCertPaths(certFile, keyFile string) {
	log.Printf("TLS: self-signed cert at %s (key: %s)", certFile, keyFile)
}



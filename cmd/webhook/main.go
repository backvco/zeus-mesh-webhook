package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/backvco/zeus-mesh-webhook/internal/webhook"
)

func main() {
	certFile := flag.String("tls-cert", "/tls/tls.crt", "TLS certificate file")
	keyFile := flag.String("tls-key", "/tls/tls.key", "TLS private key file")
	addr := flag.String("addr", ":8443", "Listen address")
	caConfigMap := flag.String("ca-configmap", "zeus-mesh-ca-bundle", "ConfigMap name containing ca.crt")
	caDir := flag.String("ca-dir", "/etc/zeus-mesh-certs", "Mount path for CA ConfigMap in injected pods")
	flag.Parse()

	// Excluded namespaces — never inject into system namespaces
	excluded := map[string]bool{
		"kube-system":      true,
		"kube-public":      true,
		"kube-node-lease":  true,
		"cert-manager":     true,
		"zeus-overlay":     true,
		"ingress-nginx":    true,
	}
	if extra := os.Getenv("EXCLUDED_NAMESPACES"); extra != "" {
		for _, ns := range splitCSV(extra) {
			excluded[ns] = true
		}
	}

	cfg := webhook.Config{
		CAConfigMap: *caConfigMap,
		CAMountDir:  *caDir,
		Excluded:    excluded,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", cfg.HandleMutate)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("load TLS keypair: %v", err)
	}

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	log.Printf("zeus-mesh-webhook listening on %s", *addr)
	if err := srv.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

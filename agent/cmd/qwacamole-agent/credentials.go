package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cztamas/qwacamole/credentials"
)

type repeated []string

func (r *repeated) String() string     { return fmt.Sprint([]string(*r)) }
func (r *repeated) Set(s string) error { *r = append(*r, s); return nil }

// Credential preparation is independent of an Nginx deployment. The same
// installed executable prepares and inspects inputs for Caddy or the agent.
func credentialCommand(cmd string, args []string, out io.Writer) error {
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	output := f.String("out", "", "new key/CSR file or new import directory (must not exist)")
	key := f.String("key", "", "local unencrypted RSA PKCS#1/PKCS#8 PEM key")
	var chain, hosts repeated
	var trust string
	var request credentials.Request
	bits := 3072
	switch cmd {
	case "keygen":
		f.IntVar(&bits, "bits", 3072, "RSA size: 2048, 3072 or 4096")
	case "csr":
		f.StringVar(&request.Country, "country", "", "organization country (two uppercase letters)")
		f.StringVar(&request.Organization, "organization", "", "registered organization name")
		f.StringVar(&request.Registration, "registration", "", "organizationIdentifier, e.g. NTRHU-123456")
		f.StringVar(&request.CommonName, "common-name", "", "subject common name")
		f.StringVar(&request.Locality, "locality", "", "organization locality")
		f.StringVar(&request.Province, "province", "", "organization state/province")
		f.StringVar(&request.Street, "street", "", "organization street address")
		f.StringVar(&request.PostalCode, "postal-code", "", "organization postal code")
		f.Var(&hosts, "hostname", "DNS SAN (repeat for every hostname)")
	case "inspect-qwac", "import-qwac":
		f.Var(&chain, "chain", "leaf-first PEM bundle or certificate (repeat in chain order)")
		f.Var(&hosts, "hostname", "binding hostname (repeat to validate every hostname)")
		f.StringVar(&trust, "trust", "", "separately selected QWAC CA PEM (exactly one certificate)")
	default:
		return fmt.Errorf("unknown credential command")
	}
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fmt.Errorf("no positional arguments accepted")
	}
	switch cmd {
	case "keygen":
		if *output == "" || *key != "" {
			return fmt.Errorf("keygen requires -out KEY; do not pass -key")
		}
		if e := credentials.GenerateKey(*output, bits); e != nil {
			return e
		}
		_, e := fmt.Fprintln(out, "created private RSA key; keep this file local and protected")
		return e
	case "csr":
		if *output == "" || *key == "" {
			return fmt.Errorf("csr requires -key KEY and -out CSR")
		}
		signer, e := credentials.ReadKey(*key)
		if e != nil {
			return e
		}
		request.Hosts = hosts
		d, e := credentials.CSR(signer, request)
		if e != nil {
			return e
		}
		if e = credentials.CreateFile(*output, d); e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, "created PKCS#10 CSR; signature verified; requested fields and extensions do not guarantee compliant issuance")
		return e
	}
	if len(chain) == 0 || *key == "" || trust == "" || len(hosts) == 0 {
		return fmt.Errorf("-chain, -key, -trust and at least one -hostname required")
	}
	if cmd == "inspect-qwac" && *output != "" {
		return fmt.Errorf("inspect-qwac writes public JSON to stdout; -out is for import-qwac")
	}
	mat, e := credentials.LoadFor(chain, *key, []string{trust}, hosts, time.Now())
	if e != nil {
		return e
	}
	leaf := mat.Chain[0]
	sum := sha256.Sum256(leaf.Raw)
	report := map[string]any{"valid": true, "leaf_sha256": hex.EncodeToString(sum[:]), "subject": leaf.Subject.String(), "dns_names": leaf.DNSNames, "validated_hostnames": []string(hosts), "not_before": leaf.NotBefore, "not_after": leaf.NotAfter, "binding_algorithm": "RS256", "chain_certificates": len(mat.Chain), "validation_scope": "local key, supported 2-QWAC profile, hostnames, validity and explicit authority chain; EU trusted-list qualification, revocation and browser trust are not established"}
	rootSum := sha256.Sum256(mat.Chain[len(mat.Chain)-1].Raw)
	report["authority_sha256"] = hex.EncodeToString(rootSum[:])
	if cmd == "import-qwac" {
		if *output == "" {
			return fmt.Errorf("import-qwac requires -out NEW_DIRECTORY")
		}
		dir, e := filepath.Abs(*output)
		if e != nil {
			return e
		}
		trustPath, e := filepath.Abs(trust)
		if e != nil {
			return e
		}
		if e = importCredential(dir, mat); e != nil {
			return e
		}
		chainPath, keyPath := filepath.Join(dir, "chain.pem"), filepath.Join(dir, "key.pem")
		report["caddy_binding_fields"] = map[string]any{"chain_files": []string{chainPath}, "key_file": keyPath, "trust_files": []string{trustPath}}
		report["agent_fields"] = map[string]any{"qwac_chain_files": []string{chainPath}, "qwac_key_file": keyPath, "qwac_trust_file": trustPath}
		report["activation"] = "configure these paths in both Caddy issuer and handler then reload, or configure the agent before startup; importing does not activate credentials or change trust"
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// Import is deliberately offline: a new private directory cannot affect the
// currently configured signer, binding or explicit trust anchor. A failure
// removes only the directory this invocation created.
func importCredential(dir string, mat *credentials.Credentials) (err error) {
	if err = os.Mkdir(dir, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	var chain []byte
	for _, c := range mat.Chain {
		chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	if err = credentials.CreateFile(filepath.Join(dir, "chain.pem"), chain); err != nil {
		return err
	}
	d, err := x509.MarshalPKCS8PrivateKey(mat.Key)
	if err != nil {
		return err
	}
	if err = credentials.CreateFile(filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: d})); err != nil {
		return err
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

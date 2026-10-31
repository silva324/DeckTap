// release-sign creates the Ed25519 key that signs DeckTap releases and signs a
// release's checksums.txt with it (see docs/release-signing.md).
//
//	release-sign keygen -out decktap-signing.key
//	release-sign sign -tag v1.2.3 -in checksums.txt -out checksums.txt.sig
//
// The private key is the base64 32-byte Ed25519 seed. It is only ever written to the
// keygen output file and read from the DECKTAP_UPDATE_SIGNING_KEY environment variable,
// never printed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/silva324/DeckTap/internal/updater"
)

// KeyEnv holds the base64 seed in CI (a GitHub Actions secret).
const KeyEnv = "DECKTAP_UPDATE_SIGNING_KEY"

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "release-sign:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: release-sign keygen -out FILE | sign -tag TAG -in checksums.txt -out checksums.txt.sig")
	}
	switch args[0] {
	case "keygen":
		return keygen(args[1:], stdout)
	case "sign":
		return sign(args[1:], getenv, stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func keygen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", "", "file to write the private key (seed) to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("keygen: -out is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // never overwrite a key
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(priv.Seed()) + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, `Private key written to %s (keep it secret; never commit it).

Public key:
  %s

Next:
  1. gh secret set %s --repo <owner>/DeckTap < %s
  2. Set updater.ReleasePublicKey in internal/updater/signature.go to the public key above.
  3. Move %s to offline storage (password manager) and delete the local copy.
`, *out, base64.StdEncoding.EncodeToString(pub), KeyEnv, *out, *out)
	return nil
}

func sign(args []string, getenv func(string) string, stdout io.Writer) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	tag := fs.String("tag", "", "release tag the signature is bound to, e.g. v1.2.3")
	in := fs.String("in", "checksums.txt", "checksums file to sign")
	out := fs.String("out", "checksums.txt.sig", "signature file to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*tag) == "" {
		return errors.New("sign: -tag is required")
	}
	priv, err := privateKeyFromEnv(getenv)
	if err != nil {
		return err
	}
	checksums, err := os.ReadFile(*in)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if len(checksums) == 0 {
		return fmt.Errorf("sign: %s is empty", *in)
	}
	sig := updater.SignChecksums(priv, *tag, checksums)
	// Self-check with the derived public key so a broken signature never ships.
	pub := priv.Public().(ed25519.PublicKey)
	if err := updater.VerifyChecksums(pub, *tag, checksums, sig); err != nil {
		return fmt.Errorf("sign: self-verification failed: %w", err)
	}
	if err := os.WriteFile(*out, sig, 0o644); err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	fmt.Fprintf(stdout, "Signed %s for %s -> %s (public key %s)\n", *in, *tag, *out, base64.StdEncoding.EncodeToString(pub))
	return nil
}

func privateKeyFromEnv(getenv func(string) string) (ed25519.PrivateKey, error) {
	v := strings.TrimSpace(getenv(KeyEnv))
	if v == "" {
		return nil, fmt.Errorf("%s is not set: refusing to publish an unsigned release", KeyEnv)
	}
	seed, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s must be the base64 %d-byte Ed25519 seed from 'release-sign keygen'", KeyEnv, ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

package tokencache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/wire"
	"golang.org/x/xerrors"
)

//go:generate mockgen -destination mock_tokencache/mock_tokencache.go github.com/int128/kubelogin/pkg/adaptors/tokencache Interface

// Set provides an implementation and interface for Kubeconfig.
var Set = wire.NewSet(
	wire.Struct(new(Repository), "*"),
	wire.Bind(new(Interface), new(*Repository)),
)

type Interface interface {
	FindByKey(dir string, key Key) (*Value, error)
	Save(dir string, key Key, value Value) error
}

// Key represents a key of a token cache.
type Key struct {
	IssuerURL      string
	ClientID       string
	ClientSecret   string
	ExtraScopes    []string
	CACertFilename string
	CACertData     string
	SkipTLSVerify  bool
}

// Value represents a value of a token cache.
type Value struct {
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Repository provides access to the token cache on the local filesystem.
// Filename of a token cache is sha256 digest of the issuer, zero-character and client ID.
type Repository struct{}

func (r *Repository) FindByKey(dir string, key Key) (*Value, error) {
	filename, err := computeFilename(key)
	if err != nil {
		return nil, xerrors.Errorf("could not compute the key: %w", err)
	}
	p := filepath.Join(dir, filename)
	f, err := os.Open(p)
	if err != nil {
		return nil, xerrors.Errorf("could not open file %s: %w", p, err)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	var c Value
	if err := d.Decode(&c); err != nil {
		return nil, xerrors.Errorf("invalid json file %s: %w", p, err)
	}
	return &c, nil
}

func (r *Repository) Save(dir string, key Key, value Value) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return xerrors.Errorf("could not create directory %s: %w", dir, err)
	}
	filename, err := computeFilename(key)
	if err != nil {
		return xerrors.Errorf("could not compute the key: %w", err)
	}
	p := filepath.Join(dir, filename)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return xerrors.Errorf("could not create file %s: %w", p, err)
	}
	defer f.Close()
	e := json.NewEncoder(f)
	if err := e.Encode(&value); err != nil {
		return xerrors.Errorf("json encode error: %w", err)
	}
	return nil
}

func computeFilename(key Key) (string, error) {
	s := sha256.New()
	e := gob.NewEncoder(s)
	if err := e.Encode(&key); err != nil {
		return "", xerrors.Errorf("could not encode the key: %w", err)
	}
	h := hex.EncodeToString(s.Sum(nil))
	return h, nil
}

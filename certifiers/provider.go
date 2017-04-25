package certifiers

import (
	"math"
	"os"

	"github.com/pkg/errors"
	wire "github.com/tendermint/go-wire"
	lc "github.com/tendermint/light-client"
	"github.com/tendermint/tendermint/types"
)

var FutureHeight = (math.MaxInt32 - 5)

// Seed is a checkpoint and the actual validator set, the base info you
// need to update to a given point, assuming knowledge of some previous
// validator set
type Seed struct {
	lc.Checkpoint
	Validators []*types.Validator
}

func (s Seed) Height() int {
	return s.Checkpoint.Height()
}

func (s Seed) Hash() []byte {
	h := s.Checkpoint.Header
	if h == nil {
		return nil
	}
	return h.ValidatorsHash
}

func (s Seed) Write(path string) (err error) {
	var f *os.File
	f, err = os.Create(path)
	if err == nil {
		var n int
		wire.WriteBinary(s, f, &n, &err)
		f.Close()
	}
	// we don't write, but this is not an error
	if os.IsExist(err) {
		return nil
	}
	return errors.WithStack(err)
}

func LoadSeed(path string) (seed Seed, err error) {
	var f *os.File
	f, err = os.Open(path)
	if err == nil {
		var n int
		wire.ReadBinaryPtr(&seed, f, 0, &n, &err)
		f.Close()
	}

	// report error nicely
	if os.IsNotExist(err) {
		err = ErrIsSeedNotFoundErr()
	} else if err != nil {
		err = errors.WithStack(err)
	}
	return
}

type Seeds []Seed

func (s Seeds) Len() int      { return len(s) }
func (s Seeds) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s Seeds) Less(i, j int) bool {
	return s[i].Height() < s[j].Height()
}

// Provider is used to get more validators by other means
//
// TODO: Also FileStoreProvider, NodeProvider, ...
type Provider interface {
	StoreSeed(seed Seed) error
	// GetByHeight returns the closest seed at with height <= h
	GetByHeight(h int) (Seed, error)
	// GetByHash returns a seed exactly matching this validator hash
	GetByHash(hash []byte) (Seed, error)
}

func LatestSeed(p Provider) (Seed, error) {
	return p.GetByHeight(FutureHeight)
}

// CacheProvider allows you to place one or more caches in front of a source
// Provider.  It runs through them in order until a match is found.
// So you can keep a local cache, and check with the network if
// no data is there.
type CacheProvider struct {
	Providers []Provider
}

func NewCacheProvider(providers ...Provider) CacheProvider {
	return CacheProvider{
		Providers: providers,
	}
}

// StoreSeed tries to add the seed to all providers.
//
// Aborts on first error it encounters (closest provider)
func (c CacheProvider) StoreSeed(seed Seed) (err error) {
	for _, p := range c.Providers {
		err := p.StoreSeed(seed)
		if err != nil {
			break
		}
	}
	return err
}

func (c CacheProvider) GetByHeight(h int) (s Seed, err error) {
	for _, p := range c.Providers {
		var ts Seed
		ts, err = p.GetByHeight(h)
		if err == nil {
			if ts.Height() > s.Height() {
				s = ts
			}
			if ts.Height() == h {
				break
			}
		}
	}
	// even if the last one had an error, if any was a match, this is good
	if s.Height() > 0 {
		err = nil
	}
	return s, err
}

func (c CacheProvider) GetByHash(hash []byte) (s Seed, err error) {
	for _, p := range c.Providers {
		s, err = p.GetByHash(hash)
		if err == nil {
			break
		}
	}
	return s, err
}

// MissingProvider doens't store anything, always a miss
// Designed as a mock for testing
type MissingProvider struct{}

func NewMissingProvider() MissingProvider {
	return MissingProvider{}
}

func (_ MissingProvider) StoreSeed(_ Seed) error { return nil }
func (_ MissingProvider) GetByHeight(_ int) (Seed, error) {
	return Seed{}, ErrIsSeedNotFoundErr()
}
func (_ MissingProvider) GetByHash(_ []byte) (Seed, error) {
	return Seed{}, ErrIsSeedNotFoundErr()
}

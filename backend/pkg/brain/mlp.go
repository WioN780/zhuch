// Package brain is a pure-Go MLP inference implementation (contracts §3).
// Weights are stored float32 little-endian, flat layer-major (per layer:
// W with shape (out,in) row-major, then bias). Forward math is float64.
package brain

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

type MLP struct {
	Sizes []int
	W     []float32
}

// ParamCount returns the flat weight-vector length for the given layer sizes.
func ParamCount(sizes []int) int {
	n := 0
	for l := 0; l < len(sizes)-1; l++ {
		n += sizes[l]*sizes[l+1] + sizes[l+1]
	}
	return n
}

// FromFlat builds an MLP from layer sizes and a flat layer-major weight vector.
func FromFlat(sizes []int, w []float32) (*MLP, error) {
	if len(sizes) < 2 {
		return nil, fmt.Errorf("brain: need at least 2 layer sizes, got %d", len(sizes))
	}
	if want := ParamCount(sizes); len(w) != want {
		return nil, fmt.Errorf("brain: sizes %v need %d weights, got %d", sizes, want, len(w))
	}
	return &MLP{Sizes: sizes, W: w}, nil
}

// Forward runs inference: tanh on hidden layers, identity output, float64 math.
func (m *MLP) Forward(in []float64) []float64 {
	cur := in
	idx := 0
	for l := 0; l < len(m.Sizes)-1; l++ {
		nin, nout := m.Sizes[l], m.Sizes[l+1]
		next := make([]float64, nout)
		for j := 0; j < nout; j++ {
			row := m.W[idx+j*nin : idx+(j+1)*nin]
			sum := float64(m.W[idx+nin*nout+j]) // bias
			for i, x := range cur {
				sum += float64(row[i]) * x
			}
			next[j] = sum
		}
		idx += nin*nout + nout
		if l < len(m.Sizes)-2 {
			for j := range next {
				next[j] = math.Tanh(next[j])
			}
		}
		cur = next
	}
	return cur
}

// modelFile is the on-disk JSON model format (contracts §3).
type modelFile struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	Sizes      []int  `json:"sizes"`
	SHA256     string `json:"sha256"`
	WeightsB64 string `json:"weights_b64"`
}

// DecodeWeightsB64 decodes base64-encoded little-endian float32 weight bytes.
func DecodeWeightsB64(s string) ([]float32, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("brain: bad base64 weights: %w", err)
	}
	return WeightsFromBytes(raw)
}

// WeightsFromBytes parses raw little-endian float32 bytes.
func WeightsFromBytes(raw []byte) ([]float32, error) {
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("brain: weight byte length %d not a multiple of 4", len(raw))
	}
	w := make([]float32, len(raw)/4)
	for i := range w {
		w[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return w, nil
}

func weightBytes(w []float32) []byte {
	raw := make([]byte, len(w)*4)
	for i, v := range w {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
	}
	return raw
}

// LoadModel reads and validates a zhuch-mlp JSON model document.
func LoadModel(r io.Reader) (*MLP, error) {
	var mf modelFile
	if err := json.NewDecoder(r).Decode(&mf); err != nil {
		return nil, fmt.Errorf("brain: bad model json: %w", err)
	}
	if mf.Format != "zhuch-mlp" || mf.Version != 1 {
		return nil, fmt.Errorf("brain: unsupported model format %q version %d", mf.Format, mf.Version)
	}
	raw, err := base64.StdEncoding.DecodeString(mf.WeightsB64)
	if err != nil {
		return nil, fmt.Errorf("brain: bad base64 weights: %w", err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != mf.SHA256 {
		return nil, fmt.Errorf("brain: weight sha256 mismatch: file says %s, bytes are %s", mf.SHA256, got)
	}
	w, err := WeightsFromBytes(raw)
	if err != nil {
		return nil, err
	}
	return FromFlat(mf.Sizes, w)
}

// SaveModel writes the zhuch-mlp JSON model document.
func (m *MLP) SaveModel(w io.Writer) error {
	raw := weightBytes(m.W)
	sum := sha256.Sum256(raw)
	return json.NewEncoder(w).Encode(modelFile{
		Format:     "zhuch-mlp",
		Version:    1,
		Sizes:      m.Sizes,
		SHA256:     hex.EncodeToString(sum[:]),
		WeightsB64: base64.StdEncoding.EncodeToString(raw),
	})
}

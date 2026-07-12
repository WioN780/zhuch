package brain

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

type golden struct {
	Sizes          []int     `json:"sizes"`
	WeightsB64     string    `json:"weights_b64"`
	Input          []float64 `json:"input"`
	ExpectedOutput []float64 `json:"expected_output"`
}

func loadGolden(t *testing.T, name string) golden {
	t.Helper()
	raw, err := os.ReadFile("../../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestForwardGolden(t *testing.T) {
	for _, name := range []string{"golden_mlp_tiny.json", "golden_mlp_full.json"} {
		t.Run(name, func(t *testing.T) {
			g := loadGolden(t, name)
			w, err := DecodeWeightsB64(g.WeightsB64)
			if err != nil {
				t.Fatal(err)
			}
			m, err := FromFlat(g.Sizes, w)
			if err != nil {
				t.Fatal(err)
			}
			out := m.Forward(g.Input)
			if len(out) != len(g.ExpectedOutput) {
				t.Fatalf("output len %d, want %d", len(out), len(g.ExpectedOutput))
			}
			for i, v := range out {
				if math.Abs(v-g.ExpectedOutput[i]) > 1e-6 {
					t.Errorf("out[%d] = %v, want %v", i, v, g.ExpectedOutput[i])
				}
			}
		})
	}
}

func TestModelRoundTrip(t *testing.T) {
	g := loadGolden(t, "golden_mlp_tiny.json")
	w, _ := DecodeWeightsB64(g.WeightsB64)
	m, _ := FromFlat(g.Sizes, w)

	var buf bytes.Buffer
	if err := m.SaveModel(&buf); err != nil {
		t.Fatal(err)
	}
	m2, err := LoadModel(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := range m.W {
		if m.W[i] != m2.W[i] {
			t.Fatalf("weight %d changed in round trip", i)
		}
	}

	// Corrupt the sha256 — must be rejected.
	var buf2 bytes.Buffer
	m.SaveModel(&buf2)
	bad := bytes.Replace(buf2.Bytes(), []byte(`"sha256":"`), []byte(`"sha256":"0`), 1)
	if _, err := LoadModel(bytes.NewReader(bad)); err == nil {
		t.Fatal("LoadModel accepted corrupted sha256")
	}
}

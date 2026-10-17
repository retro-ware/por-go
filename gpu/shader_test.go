package gpu

import (
	"encoding/binary"
	"github.com/gogpu/naga"
	"math"
	"testing"
)

func TestShaderPack(t *testing.T) {
	s, err := NewShader([]byte(`struct Uniforms { A: f32, B: vec3f, C: vec2f }
fn Fragment(dstPos: vec4f, srcPos: vec2f, color: vec4f, custom: vec4f) -> vec4f { return imageSrc0At(srcPos) * u.A; }`))
	if err != nil {
		t.Fatal(err)
	}
	b := s.pack([8]float32{1, 2, 3, 4, 5, 6}, map[string]any{"A": 7.0, "B": []float32{8, 9, 10}, "C": []float32{11, 12}})
	f := func(off int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b[off:])) }
	// builtins (32 bytes), A at 32, B aligned to 48, C at 64
	if len(b) != 72 || f(0) != 1 || f(32) != 7 || f(48) != 8 || f(56) != 10 || f(64) != 11 || f(68) != 12 {
		t.Fatalf("layout %d bytes: %v", len(b), b)
	}
	if _, err := NewShader([]byte(`fn Fragment(dstPos: vec4f, srcPos: vec2f, color: vec4f, custom: vec4f) -> vec4f { return color; }`)); err != nil {
		t.Fatal("no uniforms:", err)
	}
}

func TestBuiltinShadersCompile(t *testing.T) {
	for name, src := range map[string]string{"std": stdWGSL, "blit": blitWGSL, "mesh": meshWGSL} {
		if _, err := naga.Compile(src); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

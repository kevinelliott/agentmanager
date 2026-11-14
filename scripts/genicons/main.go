// Command genicons draws AgentManager's icons from code, so there is no
// binary source artwork to keep in sync. Run it from the repository root:
//
//	go run ./scripts/genicons
//
// It writes the tray icons and favicon that are embedded in the helper, and
// on macOS also builds resources/macos/AppIcon.icns with iconutil.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Brand gradient, top-left to bottom-right.
var gradient = [3][3]float64{
	{0x7c, 0x6c, 0xff},
	{0x4f, 0x8c, 0xff},
	{0x36, 0xc2, 0xa6},
}

const samples = 4 // supersampling per axis

// shade returns the premultiplied colour of the icon at (x, y) in the unit
// square. margin is the empty border around the rounded square.
type shade func(x, y float64) (r, g, b, a float64)

// mark is the glyph: a ring with a dot at its centre.
func mark(x, y float64) bool {
	d := math.Hypot(x-0.5, y-0.5)
	return (d >= 0.20 && d <= 0.30) || d <= 0.085
}

// appIcon is the full-colour icon: the mark in white on a gradient tile.
func appIcon(margin float64) shade {
	return func(x, y float64) (float64, float64, float64, float64) {
		// Map into the tile's own unit square.
		u := (x - margin) / (1 - 2*margin)
		v := (y - margin) / (1 - 2*margin)
		if !inRoundedSquare(u, v, 0.225) {
			return 0, 0, 0, 0
		}
		if mark(u, v) {
			return 1, 1, 1, 1
		}
		t := (u + v) / 2
		var c [3]float64
		for i := range c {
			if t < 0.5 {
				c[i] = gradient[0][i] + (gradient[1][i]-gradient[0][i])*t*2
			} else {
				c[i] = gradient[1][i] + (gradient[2][i]-gradient[1][i])*(t-0.5)*2
			}
		}
		return c[0] / 255, c[1] / 255, c[2] / 255, 1
	}
}

// templateIcon is the macOS menu bar glyph: black with alpha, tinted by the
// system for light and dark menu bars.
func templateIcon(x, y float64) (float64, float64, float64, float64) {
	// Scale the mark up to fill the canvas; a menu bar icon has no tile.
	u := 0.5 + (x-0.5)*0.68
	v := 0.5 + (y-0.5)*0.68
	if mark(u, v) {
		return 0, 0, 0, 1
	}
	return 0, 0, 0, 0
}

func inRoundedSquare(u, v, radius float64) bool {
	if u < 0 || u > 1 || v < 0 || v > 1 {
		return false
	}
	dx := math.Max(math.Max(radius-u, u-(1-radius)), 0)
	dy := math.Max(math.Max(radius-v, v-(1-radius)), 0)
	return dx*dx+dy*dy <= radius*radius
}

func render(size int, fn shade) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	n := float64(samples * samples)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/samples) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/samples) / float64(size)
					sr, sg, sb, sa := fn(x, y)
					r += sr * sa
					g += sg * sa
					b += sb * sa
					a += sa
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(r / a * 255)),
				G: uint8(math.Round(g / a * 255)),
				B: uint8(math.Round(b / a * 255)),
				A: uint8(math.Round(a / n * 255)),
			})
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writeICO writes a Windows icon with one 32-bit BMP image per size. BMP
// entries load everywhere, unlike PNG entries at small sizes.
func writeICO(path string, sizes []int, fn shade) error {
	type entry struct {
		size int
		data []byte
	}
	var entries []entry
	for _, size := range sizes {
		img := render(size, fn)
		var b bytes.Buffer
		maskRow := ((size + 31) / 32) * 4
		// BITMAPINFOHEADER; height is doubled to cover the AND mask.
		hdr := []any{
			uint32(40), int32(size), int32(size * 2), uint16(1), uint16(32),
			uint32(0), uint32(size*size*4 + maskRow*size), int32(0), int32(0), uint32(0), uint32(0),
		}
		for _, v := range hdr {
			_ = binary.Write(&b, binary.LittleEndian, v)
		}
		for y := size - 1; y >= 0; y-- { // bottom-up BGRA
			for x := 0; x < size; x++ {
				c := img.NRGBAAt(x, y)
				b.Write([]byte{c.B, c.G, c.R, c.A})
			}
		}
		b.Write(make([]byte, maskRow*size)) // AND mask: alpha channel is used instead
		entries = append(entries, entry{size, b.Bytes()})
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, []uint16{0, 1, uint16(len(entries))})
	offset := 6 + 16*len(entries)
	for _, e := range entries {
		out.Write([]byte{byte(e.size), byte(e.size), 0, 0})
		_ = binary.Write(&out, binary.LittleEndian, []uint16{1, 32})
		_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(e.data)), uint32(offset)})
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeICNS(path string) error {
	dir, err := os.MkdirTemp("", "agentmgr-icons")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	set := filepath.Join(dir, "AppIcon.iconset")

	// macOS app icons leave about 10% of the canvas empty around the tile.
	fn := appIcon(0.10)
	for _, base := range []int{16, 32, 128, 256, 512} {
		if err := writePNG(filepath.Join(set, fmt.Sprintf("icon_%dx%d.png", base, base)), render(base, fn)); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(set, fmt.Sprintf("icon_%dx%d@2x.png", base, base)), render(base*2, fn)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := exec.Command("iconutil", "-c", "icns", "-o", path, set).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iconutil: %w: %s", err, out)
	}
	return nil
}

func main() {
	icons := filepath.Join("internal", "systray", "icons")
	steps := []struct {
		name string
		run  func() error
	}{
		{"macOS menu bar icon", func() error { return writePNG(filepath.Join(icons, "tray_template.png"), render(64, templateIcon)) }},
		{"Linux tray icon", func() error { return writePNG(filepath.Join(icons, "tray.png"), render(64, appIcon(0))) }},
		{"Windows tray icon", func() error { return writeICO(filepath.Join(icons, "tray.ico"), []int{16, 24, 32, 48}, appIcon(0)) }},
		{"window favicon", func() error {
			return writePNG(filepath.Join("internal", "systray", "webui", "icon.png"), render(128, appIcon(0)))
		}},
	}
	if runtime.GOOS == "darwin" {
		steps = append(steps, struct {
			name string
			run  func() error
		}{"macOS app icon", func() error { return writeICNS(filepath.Join("resources", "macos", "AppIcon.icns")) }})
	} else {
		fmt.Println("skipping resources/macos/AppIcon.icns: needs iconutil on macOS")
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", s.name, err)
			os.Exit(1)
		}
		fmt.Println("wrote", s.name)
	}
}

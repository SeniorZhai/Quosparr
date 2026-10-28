package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestProviderCatalog(t *testing.T) {
	const root = "assets/providers/catalog"
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct{ ID, SHA256 string }
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 110 {
		t.Fatalf("catalog has %d entries", len(entries))
	}
	icons := make(map[string]*ui.SVG, len(entries))
	for _, e := range entries {
		if _, ok := icons[e.ID]; ok {
			t.Fatalf("duplicate icon: %s", e.ID)
		}
		data, err := os.ReadFile(filepath.Join(root, e.ID+".svg"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != e.SHA256 {
			t.Fatalf("checksum mismatch: %s", e.ID)
		}
		icon, err := ui.ParseSVG(data)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		icons[e.ID] = icon
	}
	for _, dark := range []bool{false, true} {
		tt := ui.NewTester(func(c *ui.Context) {
			theme := applyTheme(c)
			ui.Column(c).Fill().Background(theme.Background).Padding(12).Children(func() {
				for start := 0; start < len(entries); start += 10 {
					ui.Row(c).Children(func() {
						for _, e := range entries[start:min(start+10, len(entries))] {
							ui.Column(c).Width(96).Height(84).AlignItems(ui.Center).Gap(4).Children(func() {
								ui.Icon(c, icons[e.ID]).Size(48, 48).TextColor(theme.Text).Label("catalog-" + e.ID)
								ui.Text(c, e.ID).FontSize(10).TextColor(theme.Text)
							})
						}
					})
				}
			})
		}, 984, 948)
		tt.SetDark(dark)
		img := tt.Image()
		bg := img.RGBAAt(0, 0)
		for _, e := range entries {
			box, ok := tt.Find("catalog-" + e.ID)
			if !ok {
				t.Fatalf("missing catalog image: %s", e.ID)
			}
			pixels := 0
			for y := int(box.Y); y < int(box.Y+box.H); y++ {
				for x := int(box.X); x < int(box.X+box.W); x++ {
					if px := img.RGBAAt(x, y); px != bg {
						pixels++
					}
				}
			}
			if pixels < 10 {
				t.Fatalf("invisible icon %s (dark=%v)", e.ID, dark)
			}
		}
		if dir := os.Getenv("RUNWAY_PROVIDER_QA_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			name := "provider-catalog-light.png"
			if dark {
				name = "provider-catalog-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, img)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("preview: %v %v", err, closeErr)
			}
		}
	}
}

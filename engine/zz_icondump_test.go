package engine

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestZZIconDump(t *testing.T) {
	out := os.Getenv("ICONDUMP")
	if out == "" {
		t.Skip()
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	e.Init()
	e.Cfg.SaveDir = "../data/save/saves/back-for-good"
	for i := 1; i <= 6; i++ {
		c := &Character{}
		if !e.LoadCharacter(fmt.Sprintf("CHRDATA%d", i), c, true) {
			t.Fatal(i)
		}
		fmt.Printf("\n== %d %s  race=%s class=%s sex=%d align=%d levels=%v hp=%d str=%d\n", i, c.Name(), raceNames[c.Race], className(c.Class), c.Sex, c.Alignment, c.Levels, c.MaxHP, c.Stats[0])
		fmt.Printf("   iconHead=%d iconBody=%d iconSize=%d colours=%x porthead=%d portbody=%d order=%d\n", c.IconHead, c.IconBody, c.IconSize, c.IconColour, c.PortHead, c.PortBody, c.Order)
		for _, it := range c.Items {
			e.ItemName(c, it, 0, 0, false, false)
			fmt.Printf("   item type=%d %q readied=%d plus=%d magic=%d\n", it.Type, it.Name(), it.Readied, int8(it.Plus), it.Magic)
		}
		c.Order = byte(i)
		e.BuildCharIcon(c, true)
		for pose := 0; pose < 2; pose++ {
			im := e.icons[c.Order][pose]
			if im == nil {
				fmt.Println("   no icon pose", pose)
				continue
			}
			src := e.picRGBA(im, 0, false, -1)
			b := src.Bounds()
			k := 8
			big := image.NewRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
			for y := 0; y < b.Dy()*k; y++ {
				for x := 0; x < b.Dx()*k; x++ {
					big.Set(x, y, src.At(x/k, y/k))
				}
			}
			_ = draw.Draw
			f, _ := os.Create(filepath.Join(out, fmt.Sprintf("orig%d_%s_pose%d.png", i, c.Name(), pose)))
			png.Encode(f, big)
			f.Close()
			fmt.Printf("   icon pose %d: %dx%d\n", pose, b.Dx(), b.Dy())
		}
	}
}

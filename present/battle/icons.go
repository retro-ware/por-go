package battle

import (
	"embed"
	"image/png"
	"io/fs"
	"strings"

	"gpu"
)

//go:embed icons/*.png
var iconFiles embed.FS

var icons = map[string]*gpu.Image{} // by button word; nil: no icon, the letter shows

// iconPath is the icon file for the button named word ("": none).
func iconPath(word string) string {
	name := strings.ToLower(word)
	if word == "Cancel" {
		name = "end"
	}
	p := "icons/" + name + ".png"
	if _, err := fs.Stat(iconFiles, p); err != nil {
		return ""
	}
	return p
}

// icon is the art for the button named word (nil: none).
func icon(word string) *gpu.Image {
	if im, ok := icons[word]; ok {
		return im
	}
	var im *gpu.Image
	if p := iconPath(word); p != "" {
		if f, err := iconFiles.Open(p); err == nil {
			if pic, err := png.Decode(f); err == nil {
				im = gpu.NewImageFromImage(pic)
			}
			f.Close()
		}
	}
	icons[word] = im
	return im
}

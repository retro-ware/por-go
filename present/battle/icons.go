package battle

import (
	"embed"
	"image"
	"image/png"
	"strings"
)

//go:embed icons/*.png
var iconFiles embed.FS

var icons = map[string]image.Image{} // by button word; nil: no icon, the letter shows

// iconPicture is the art for the button named word (nil: none).
func iconPicture(word string) image.Image {
	if im, ok := icons[word]; ok {
		return im
	}
	name := strings.ToLower(word)
	if word == "Cancel" {
		name = "end"
	}
	var im image.Image
	if f, err := iconFiles.Open("icons/" + name + ".png"); err == nil {
		im, _ = png.Decode(f)
		f.Close()
	}
	icons[word] = im
	return im
}

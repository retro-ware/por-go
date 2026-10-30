package engine

import "image"

// The combat icon from a PNG (extra): a file dropped on the icon editor,
// saved as NAME.icon.png beside the record. A square picture is both
// poses; one twice as wide as high is cut in the middle, the left half the
// ready pose, the right the action pose. The battle draws it in place of
// the built icon; the page's icons (the overview) stay as built.

// setIconPNG takes im as c's combat icon; false when it is neither square
// nor twice as wide as high (c keeps what it had).
func (c *Character) setIconPNG(im image.Image) bool {
	if b := im.Bounds(); b.Dx() != b.Dy() && b.Dx() != 2*b.Dy() {
		return false
	}
	c.Icon, c.iconPoses = im, [2][2]image.Image{}
	return true
}

// iconPNG is c's PNG icon for pose (0 ready, 1 action), flipped to face
// left when flip; nil without a PNG. The cuts are kept: the battle's
// figures are cached by image.
func (c *Character) iconPNG(pose int, flip bool) image.Image {
	if c.Icon == nil {
		return nil
	}
	f := 0
	if flip {
		f = 1
	}
	if b := c.Icon.Bounds(); b.Dx() == b.Dy() { // one picture for both poses
		pose = 0
	}
	if c.iconPoses[pose][f] == nil {
		c.iconPoses[pose][f] = iconPose(c.Icon, pose, flip)
	}
	return c.iconPoses[pose][f]
}

// iconPose cuts pose (0 left, 1 right; the same square when im is square)
// out of im, mirrored when flip.
func iconPose(im image.Image, pose int, flip bool) image.Image {
	b := im.Bounds()
	w := b.Dy()
	x0 := b.Min.X + pose*(b.Dx()-w)
	out := image.NewRGBA(image.Rect(0, 0, w, w))
	for y := 0; y < w; y++ {
		for x := 0; x < w; x++ {
			sx := x0 + x
			if flip {
				sx = x0 + w - 1 - x
			}
			out.Set(x, y, im.At(sx, b.Min.Y+y))
		}
	}
	return out
}

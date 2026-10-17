// items-html writes an overview page of every weapon and armor piece in the catalog.
// Usage: go run ./tools/items-html > docs/items.html
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"

	"blobber/party"
)

func main() {
	var names []string
	for n, it := range party.Catalog {
		if it.Kind == party.Weapon || it.Kind == party.Armor || it.Kind == party.Accessory {
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := party.Catalog[names[i]], party.Catalog[names[j]]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Slot != b.Slot {
			return a.Slot < b.Slot
		}
		return a.Value < b.Value || a.Value == b.Value && names[i] < names[j]
	})
	fmt.Println(`<!doctype html><meta charset=utf-8><title>Blobber items</title>
<style>body{font:14px system-ui;background:#1b1612;color:#e8d8b8;margin:24px}
table{border-collapse:collapse}td,th{padding:3px 10px;text-align:left;border-bottom:1px solid #332a22}
img{width:48px;height:48px;image-rendering:pixelated;vertical-align:middle}
h2{margin-top:32px;color:#d9a441}</style>`)
	last := ""
	for _, n := range names {
		it := party.NewItem(n, 0)
		if h := party.KindNames[it.Kind] + " · " + string(it.Slot); h != last {
			if last != "" {
				fmt.Println("</table>")
			}
			fmt.Printf("<h2>%s</h2><table><tr><th></th><th>Name</th><th>Script name</th><th>Dmg</th><th>AC</th><th>Value</th></tr>\n", h)
			last = h
		}
		b, _ := os.ReadFile("ui/assets/items/" + it.Icon + ".png")
		fmt.Printf("<tr><td><img src='data:image/png;base64,%s'></td><td>%s</td><td><code>%s</code></td><td>%s</td><td>%s</td><td>%d</td></tr>\n",
			base64.StdEncoding.EncodeToString(b), it.Name(), n, nz(it.Damage), nz(it.AC), it.Value)
	}
	fmt.Println("</table>")
}

func nz(v int) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprint(v)
}

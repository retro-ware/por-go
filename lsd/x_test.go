package lsd
import ("testing";"fmt";"por/dax")
func TestGeo(t *testing.T){
 l,_:=dax.NewLoader("../data/dos")
 b:=l.Block("GEO3",0)
 g:=b[2:]
 for y:=0;y<16;y++{ s:=""; for x:=0;x<16;x++{ c:=g[0x200+y*16+x]; if c==0 {s+=" ."} else {s+=fmt.Sprintf("%2d",c)}}; fmt.Println(s)}
}

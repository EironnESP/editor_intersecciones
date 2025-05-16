package layouts

import (
	"fyne.io/fyne/v2"
)

type CarrilesHorizontales struct {
}

var distanciasH = []float32{0, 70, 0, 44, 0, 26, 0}
var margenesH = []float32{0, 6, 0, 1, 0, 0, 0}

func (d *CarrilesHorizontales) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)

	for _, o := range objects {
		childSize := o.MinSize()

		if childSize.Width > w {
			w = childSize.Width
		}
		if childSize.Height > h {
			h = childSize.Height
		}
	}
	return fyne.NewSize(w, h)
}

func (d *CarrilesHorizontales) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	pos := fyne.NewPos(0, 0)

	if len(objects) == 1 {
		objects[0].Move(fyne.NewPos(64, 30))
	} else {
		for i, o := range objects {
			if i%2 == 0 { //flecha
				pos.Y = 30
				pos.X += margenesH[len(objects)-2]
				o.Move(pos)
				pos.X += distanciasH[len(objects)-2]
				pos.X += margenesH[len(objects)-2]
			} else { //linea
				pos.Y = 0
				o.Move(pos)
				pos.X += 30
			}
		}
	}
}

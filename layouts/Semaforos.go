package layouts

import (
	"fyne.io/fyne/v2"
)

type Semaforos struct {
}

func (d *Semaforos) MinSize(objects []fyne.CanvasObject) fyne.Size {
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

func (d *Semaforos) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	switch len(objects) {
	case 0:
		return
	case 1:
		objects[0].Move(fyne.NewPos(30, 0))
	case 2:
		objects[0].Move(fyne.NewPos(16, 0))
		objects[1].Move(fyne.NewPos(46, 0))
	case 3:
		objects[0].Move(fyne.NewPos(0, 0))
		objects[1].Move(fyne.NewPos(30, 0))
		objects[2].Move(fyne.NewPos(60, 0))
	case 4:
		//peatonal
	}
}

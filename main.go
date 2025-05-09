package main

import (
	"database/sql"
	"fmt"
	"image"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/kbinani/screenshot"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	a := app.New()

	//VARIABLES GLOBALES
	home, _ := os.UserHomeDir()
	semaforoSize := fyne.NewSize(28, 100)
	modoEdicion := false
	numDirecciones := 0

	//INICIALIZACION BBDD, ¿PREGUNTAR O COMPROBAR?
	err := inicializarDB(home)
	if err != nil {
		mostrarError("Error al inicializar la base de datos: "+err.Error(), a)
		a.Run()
		return
	}

	//INICIALIZACION PANTALLA INICIAL
	w := a.NewWindow("Editor de intersecciones")
	w.Resize(windowSize(1)) // 1 = 100% de la pantalla
	w.CenterOnScreen()

	//CARGAR ICONO
	icono, err := fyne.LoadResourceFromPath("images/icono.png")
	if err != nil {
		mostrarError("Error al cargar el icono: "+err.Error(), a)
		a.Run()
		return
	}
	w.SetIcon(icono)

	//CARGAR IMAGENES
	/*
		semVerde, err := getImagen("images/semaforos/semVerde.png")
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
	*/

	semAmbar, err := getImagen("images/semaforos/semAmbar.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	/*
		semRojo, err := getImagen("images/semaforos/semRojo.png")
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
	*/
	semApagado, err := getImagen("images/semaforos/semApagado.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}

	//FONDO
	image := canvas.NewImageFromFile("images/cruces/cruce_vacio.png")
	image.FillMode = canvas.ImageFillOriginal

	semTest := canvas.NewImageFromImage(semAmbar)
	semTest.FillMode = canvas.ImageFillOriginal
	semTest.Resize(semaforoSize)

	layoutBotonesEditar := container.NewWithoutLayout()
	layoutBotonesEditar.Resize(fyne.NewSize(994, 993))

	layoutComponentes := container.NewWithoutLayout()
	layoutComponentes.Resize(fyne.NewSize(994, 993))
	layoutComponentes.Add(semTest)

	fondo := container.NewCenter(container.NewStack(image, layoutComponentes, layoutBotonesEditar))
	go ambar(semAmbar, semApagado, semTest, layoutComponentes)

	//BARRAS DE HERRAMIENTAS
	barraHerramientasEdicion := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() {
			//HACER EN ACCESO A DATOS
			fmt.Println("guardar diseño")
			fmt.Println(layoutBotonesEditar.Size())
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			if modoEdicion {
				modoEdicion = false
				colocarBotones(nil, layoutBotonesEditar, nil, numDirecciones, true, modoEdicion)
			} else {
				modoEdicion = true
				colocarBotones(nil, layoutBotonesEditar, nil, numDirecciones, true, modoEdicion)
			}
		}),
		widget.NewToolbarAction(theme.VisibilityIcon(), func() {
			fmt.Println("cambiar orientacion")
		}),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			//DISTRIBUCION
			fmt.Println("ayuda")
		}),
	)

	barraHerramientasEjecucion := widget.NewToolbar(
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() {}),
		widget.NewToolbarAction(theme.MediaPauseIcon(), func() {}),
		widget.NewToolbarAction(theme.MediaReplayIcon(), func() {}),
	)

	contentEdicion := container.NewBorder(barraHerramientasEdicion, nil, nil, nil, fondo)
	contentEjecucion := container.NewBorder(barraHerramientasEjecucion, nil, nil, nil, fondo)

	tabs := container.NewAppTabs(
		container.NewTabItem("Edición", contentEdicion),
		container.NewTabItem("Ejecución", contentEjecucion),
	)

	w.SetContent(tabs)
	w.Show()

	//DIALOG NUEVO DISEÑO O EXISTENTE
	var c *fyne.Container
	var d *dialog.CustomDialog
	var dDir *dialog.CustomDialog

	botonNuevo := widget.NewButton("Nuevo diseño", func() {
		fmt.Println("nuevo")

		boton3 := widget.NewButton("3 direcciones", func() {
			numDirecciones = 3
			image.File = "images/cruces/cruce3_vacio.png"
			image.Refresh()
			colocarBotones(w, layoutBotonesEditar, layoutComponentes, numDirecciones, false, modoEdicion)
			dDir.Hide()
		})
		boton4 := widget.NewButton("4 direcciones", func() {
			numDirecciones = 4
			colocarBotones(w, layoutBotonesEditar, layoutComponentes, numDirecciones, false, modoEdicion)
			dDir.Hide()
		})

		c = container.New(layout.NewVBoxLayout(), boton3, boton4)

		dDir = dialog.NewCustomWithoutButtons("Tipo de intersección", c, w)
		d.Hide()
		dDir.Show()
	})

	//HACER EN ACCESO A DATOS
	botonAbrir := widget.NewButton("Abrir diseño guardado", func() {
		fmt.Println("abrir")

		db, err := sql.Open("sqlite3", home+"/.intersecciones/test.db")
		if err != nil {
			mostrarError("Error al abrir la base de datos: "+err.Error(), a)
			return
		}
		defer db.Close()

		query := "SELECT nombre, COUNT(*) FROM Intersecciones"
		rows, err := db.Query(query)
		if err != nil {
			mostrarError("Error al consultar la base de datos: "+err.Error(), a)
			return
		}
		c.Objects = c.Objects[:0] //limpiar container

		var nombre string
		var num int

		for rows.Next() {
			err = rows.Scan(&nombre, &num)
			if num == 0 {
				mostrarError("No hay diseños guardados", a, w)
				d.Hide()
				return
			}

			if err != nil {
				mostrarError("Error al leer la base de datos: "+err.Error(), a)
				return
			}

			fmt.Println(nombre)
			c.Objects = append(c.Objects, widget.NewButton(nombre, func() {
				d.Hide()
				fmt.Println("abrir diseño: " + nombre)

			}))
		}
	})

	c = container.New(layout.NewVBoxLayout(), botonNuevo, botonAbrir)

	d = dialog.NewCustomWithoutButtons("Abrir diseño", c, w)
	d.Show()

	//POSICIONAR
	fmt.Println(1, layoutBotonesEditar.Size(), fondo.Size(), image.Size())

	layoutBotonesEditar.Resize(fyne.NewSize(994, 993))
	fmt.Println(2, layoutBotonesEditar.Size(), fondo.Size(), image.Size())

	a.Run()
}

// VENTANA DE ERROR
func mostrarError(e string, a fyne.App, w ...fyne.Window) {
	wError := a.NewWindow("Error")
	wError.CenterOnScreen()
	icono, err := fyne.LoadResourceFromPath("images/icono.png")
	if err != nil {
		log.Fatal("Error al cargar el icono: ", err)
	}
	wError.SetIcon(icono)

	image := canvas.NewImageFromFile("images/error100.png")
	image.FillMode = canvas.ImageFillOriginal

	wError.SetContent(container.New(layout.NewVBoxLayout(), container.New(layout.NewHBoxLayout(), image, container.NewCenter(widget.NewLabel(e))), widget.NewButton("Aceptar", func() {
		wError.Close()
	}),
	))
	wError.Show()
	if w != nil {
		w[0].Close()
	}
	wError.SetOnClosed(func() {
		a.Quit()
	},
	)
}

func inicializarDB(home string) error {
	if _, err := os.Stat(home + "/.intersecciones/"); os.IsNotExist(err) { //futuro: comprobacion version actualizar estructura bbdd
		err := os.MkdirAll(home+"/.intersecciones/", 0755)
		if err != nil {
			return err
		}
		_, err = os.Create(home + "/.intersecciones/test.db")
		if err != nil {
			return err
		}

		db, err := sql.Open("sqlite3", home+"/.intersecciones/test.db")

		if err != nil {
			return err
		}

		defer db.Close()

		s := `
		DROP TABLE IF EXISTS Intersecciones;
		CREATE TABLE Intersecciones(
			id INTEGER PRIMARY KEY,
			nombre TEXT, 
			fecha_ult_mod TEXT,
			num_direcciones INTEGER
			);

		DROP TABLE IF EXISTS Direcciones;
		CREATE TABLE Direcciones(
			id INTEGER PRIMARY KEY,
			interseccion_id INTEGER,
			sentido INTEGER,
			tiene_semaforos INTEGER,
			tiene_paso_peatones INTEGER,
			FOREIGN KEY(interseccion_id) REFERENCES Intersecciones(id)
			ON DELETE CASCADE ON UPDATE CASCADE
			);



			
		DROP TABLE IF EXISTS Carriles;
		CREATE TABLE Carriles(
			id INTEGER PRIMARY KEY,
			interseccion_id INTEGER,
			direccion_id INTEGER,
			sentido_in_out INTEGER,
			sentido_giro INTEGER,
			posicion INTEGER,
			FOREIGN KEY(interseccion_id) REFERENCES Intersecciones(id)
			ON DELETE CASCADE ON UPDATE CASCADE,
			FOREIGN KEY(direccion_id) REFERENCES Direcciones(id)
			ON DELETE CASCADE ON UPDATE CASCADE
			);
		`
		_, err = db.Exec(s)

		if err != nil {
			return err
		}

		fmt.Println("BBDD creada correctamente")
		return nil
	} else {
		fmt.Println("La base de datos ya existe")
		return nil
	}
}

func windowSize(part float32) fyne.Size {
	if screenshot.NumActiveDisplays() > 0 {
		bounds := screenshot.GetDisplayBounds(0) //0 = monitor 1
		return fyne.NewSize(float32(bounds.Dx())*part, float32(bounds.Dy())*part)
	}
	return fyne.NewSize(800, 800)
}

func getPosicion(a interface{}, c *fyne.Container) int {
	for i, obj := range c.Objects {
		if obj == a {
			return i
		}
	}
	return -1
}

func getImagen(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	return image, err
}

func ambar(ambar, apagado image.Image, sem *canvas.Image, c *fyne.Container) {
	pos := getPosicion(sem, c)
	tick := time.NewTicker(time.Second)

	for i := 0; i < 10; i++ {
		imagen := c.Objects[pos].(*canvas.Image)

		imagen.Image = ambar
		fyne.Do(func() { imagen.Refresh() })
		<-tick.C

		imagen.Image = apagado
		fyne.Do(func() { imagen.Refresh() })
		<-tick.C
	}
}

func colocarBotones(w fyne.Window, cBotones, cElementos *fyne.Container, numDir int, inicializado, editar bool) {
	if !inicializado {
		for i := 0; i < numDir; i++ {
			boton := widget.NewButton("Editar "+fmt.Sprint(i+1), func() {
				menuEdicion(i+1, w, cElementos)
			})
			boton.Hidden = true
			cBotones.Add(boton)
			boton.Resize(fyne.NewSize(80, 30))
			switch i { //497 es el centro, la posicion de los botones es su esquina superior izquierda
			case 0:
				boton.Move(fyne.NewPos(357, 482))
			case 1:
				boton.Move(fyne.NewPos(457, 582))
			case 2:
				boton.Move(fyne.NewPos(557, 482))
			case 3:
				boton.Move(fyne.NewPos(457, 382))
			}
		}
	} else {
		fmt.Println("mostrando:", editar)

		for _, boton := range cBotones.Objects {
			boton := boton.(*widget.Button)
			boton.Hidden = !editar
			boton.Refresh()
		}
	}
}

func menuEdicion(dir int, w fyne.Window, parent *fyne.Container) {
	c := container.New(layout.NewVBoxLayout())

	//PASO DE PEATONES
	var checkPasoPeatones *widget.Check
	pasoPeatonesActivado := false

	switch dir {
	case 1:
		for _, obj := range parent.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				fmt.Println("dsdsdsa", obj.Position())
				if obj.Position() == fyne.NewPos(297, 360) || obj.Position() == fyne.NewPos(297, 438) {
					fmt.Println("weon")
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 2:
		for _, obj := range parent.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(360, 597) || obj.Position() == fyne.NewPos(438, 597) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 3:
		for _, obj := range parent.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(597, 360) || obj.Position() == fyne.NewPos(597, 438) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 4:
		for _, obj := range parent.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(360, 297) || obj.Position() == fyne.NewPos(438, 297) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	}

	checkPasoPeatones = widget.NewCheck("Paso de peatones", func(b bool) {
		addPasoPeatones(dir, b, parent)
	})

	c.Add(checkPasoPeatones)
	if pasoPeatonesActivado {
		checkPasoPeatones.SetChecked(true)
	}

	//modificar semaforos
	//modificar num carriles
	//direccion ^v
	//direccion <>

	d := dialog.NewCustom(fmt.Sprintf("Editar dirección %d", dir), "Cerrar", c, w)

	d.Show()
}

func addPasoPeatones(dir int, b bool, parent *fyne.Container) {
	if b {
		pasoPeatonesImg, err := getImagen("images/marcas_viales/paso.png")
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), fyne.CurrentApp())
		}

		pasoPeatonesImgRotada, err := getImagen("images/marcas_viales/paso_rotado.png")
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), fyne.CurrentApp())
		}

		pasoPeatones1 := canvas.NewImageFromImage(pasoPeatonesImg)
		pasoPeatones1.FillMode = canvas.ImageFillOriginal
		pasoPeatones1.Resize(fyne.NewSize(100, 195))

		pasoPeatones2 := canvas.NewImageFromImage(pasoPeatonesImg)
		pasoPeatones2.FillMode = canvas.ImageFillOriginal
		pasoPeatones2.Resize(fyne.NewSize(100, 195))

		pasoPeatonesRotado1 := canvas.NewImageFromImage(pasoPeatonesImgRotada)
		pasoPeatonesRotado1.FillMode = canvas.ImageFillOriginal
		pasoPeatonesRotado1.Resize(fyne.NewSize(195, 100))

		pasoPeatonesRotado2 := canvas.NewImageFromImage(pasoPeatonesImgRotada)
		pasoPeatonesRotado2.FillMode = canvas.ImageFillOriginal
		pasoPeatonesRotado2.Resize(fyne.NewSize(195, 100))

		switch dir {
		case 1:
			pasoPeatones1.Move(fyne.NewPos(297, 360))
			pasoPeatones2.Move(fyne.NewPos(297, 438))

			parent.Add(pasoPeatones1)
			parent.Add(pasoPeatones2)
		case 2:
			pasoPeatonesRotado1.Move(fyne.NewPos(360, 597))
			pasoPeatonesRotado2.Move(fyne.NewPos(438, 597))

			parent.Add(pasoPeatonesRotado1)
			parent.Add(pasoPeatonesRotado2)
		case 3:
			pasoPeatones1.Move(fyne.NewPos(597, 360))
			pasoPeatones2.Move(fyne.NewPos(597, 438))

			parent.Add(pasoPeatones1)
			parent.Add(pasoPeatones2)
		case 4:
			pasoPeatonesRotado1.Move(fyne.NewPos(360, 297))
			pasoPeatonesRotado2.Move(fyne.NewPos(438, 297))

			parent.Add(pasoPeatonesRotado1)
			parent.Add(pasoPeatonesRotado2)
		}

		parent.Refresh()
	} else {
		var objetosAEliminar []fyne.CanvasObject
		for _, obj := range parent.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				switch obj.Position() {
				case fyne.NewPos(297, 360), fyne.NewPos(297, 438):
					if dir == 1 {
						objetosAEliminar = append(objetosAEliminar, obj)
					}
				case fyne.NewPos(360, 597), fyne.NewPos(438, 597):
					if dir == 2 {
						objetosAEliminar = append(objetosAEliminar, obj)
					}
				case fyne.NewPos(597, 360), fyne.NewPos(597, 438):
					if dir == 3 {
						objetosAEliminar = append(objetosAEliminar, obj)
					}
				case fyne.NewPos(360, 297), fyne.NewPos(438, 297):
					if dir == 4 {
						objetosAEliminar = append(objetosAEliminar, obj)
					}
				}
			}
		}

		// Elimina los objetos recopilados
		for _, obj := range objetosAEliminar {
			parent.Remove(obj)
		}

		parent.Refresh()
		fmt.Println("removiendo")
	}
}

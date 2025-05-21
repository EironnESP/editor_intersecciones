package main

import (
	"bytes"
	"database/sql"
	"embed"
	"fmt"
	"image"
	"os"
	"slices"
	"strconv"
	"time"

	"Editor_Intersecciones/layouts"

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

//go:embed images/*
var images embed.FS

// VARIABLES GLOBALES
var home, _ = os.UserHomeDir()
var modoEdicion = false
var flechas []image.Image
var semaforos []image.Image
var otrasMarcas []image.Image
var layoutsMarcas = make([]*fyne.Container, 4)
var layoutsSemaforos = make([]*fyne.Container, 8)
var semaforoSize = fyne.NewSize(28, 100)
var numDirecciones = 0
var secuencias = make([][]Secuencia, 5)
var botonesSemaforos = make([][]*widget.Button, 5)
var layoutComponentes *fyne.Container
var colores = []string{"Verde", "Ámbar", "Ámbar (parpadeo)", "Rojo"}
var fasesCopiada Secuencia

func main() {
	a := app.New()

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
	i, err := images.ReadFile("images/icono.png")
	if err != nil {
		mostrarError("Error al cargar el icono: "+err.Error(), a)
		a.Run()
		return
	}
	icono := fyne.NewStaticResource("icono", i)
	w.SetIcon(icono)

	//CARGAR IMAGENES
	cargarSemaforos(a)
	cargarFlechas(a)
	cargarOtrasMarcas(a)

	fondo4, err := getImagen("images/cruces/cruce_vacio.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}

	fondo3, err := getImagen("images/cruces/cruce3_vacio.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}

	//FONDO
	image := canvas.NewImageFromImage(fondo4)
	image.FillMode = canvas.ImageFillOriginal

	layoutBotonesEditar := container.NewWithoutLayout()
	layoutBotonesEditar.Resize(fyne.NewSize(994, 993))

	layoutComponentes = container.NewWithoutLayout()
	layoutComponentes.Resize(fyne.NewSize(994, 993))

	fondo := container.NewCenter(container.NewStack(image, layoutComponentes, layoutBotonesEditar))

	//BARRAS DE HERRAMIENTAS
	barraHerramientasEdicion := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() {
			//HACER EN ACCESO A DATOS
			fmt.Println("guardar diseño")
			fmt.Println(secuencias)
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			if modoEdicion {
				modoEdicion = false
				colocarBotones(nil, layoutBotonesEditar, numDirecciones, true, modoEdicion)
			} else {
				modoEdicion = true
				colocarBotones(nil, layoutBotonesEditar, numDirecciones, true, modoEdicion)
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

	contentEdicion := container.NewBorder(nil, nil, container.NewVBox(barraHerramientasEdicion), nil, fondo)
	contentEjecucion := container.NewBorder(nil, nil, container.NewVBox(barraHerramientasEjecucion), nil, fondo)

	tabs := container.NewAppTabs(
		container.NewTabItem("Edición", contentEdicion),
		container.NewTabItem("Ejecución", contentEjecucion),
	)

	tabs.OnSelected = func(t *container.TabItem) {
		if t.Text == "Ejecución" { //OCULTAR BOTONES EN VISTA DE EJECUCION
			modoEdicion = false
			colocarBotones(nil, layoutBotonesEditar, numDirecciones, true, modoEdicion)
		}
	}

	tabs.SetTabLocation(container.TabLocationLeading)

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
			image.Image = fondo3
			image.Refresh()
			colocarBotones(w, layoutBotonesEditar, numDirecciones, false, modoEdicion)
			dDir.Hide()
		})
		boton4 := widget.NewButton("4 direcciones", func() {
			numDirecciones = 4
			colocarBotones(w, layoutBotonesEditar, numDirecciones, false, modoEdicion)
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
	layoutBotonesEditar.Resize(fyne.NewSize(994, 993))

	a.Run()
}

// VENTANA DE ERROR
func mostrarError(e string, a fyne.App, w ...fyne.Window) {
	wError := a.NewWindow("Error")
	wError.CenterOnScreen()
	i, err := images.ReadFile("images/icono.png")
	if err != nil {
		mostrarError("Error al cargar el icono: "+err.Error(), a)
		a.Run()
		return
	}
	icono := fyne.NewStaticResource("icono", i)
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

func getPosicionEnContainer(a interface{}, c *fyne.Container) int {
	for i, obj := range c.Objects {
		if obj == a {
			return i
		}
	}
	return -1
}

func getPosicionEnArray(a *canvas.Image, b []fyne.CanvasObject) int {
	for i, obj := range b {
		if obj == a {
			return i
		}
	}
	return -1
}

func getPosicionFlecha(a image.Image, b []image.Image) int {
	for i, obj := range b {
		if obj == a {
			return i
		}
	}
	return -1
}

func getImagen(path string) (image.Image, error) {
	data, err := images.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error al leer la imagen: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("error al decodificar la imagen: %w", err)
	}

	return img, nil
}

func cargarSemaforos(a fyne.App) {
	f, err := getImagen("images/semaforos/semApagado.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semVerde.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semAmbar.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semRojo.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semVerdeDcha.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semAmbarDcha.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semRojoDcha.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semVerdeFrente.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semAmbarFrente.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semRojoFrente.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semVerdeIzqda.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semAmbarIzqda.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)

	f, err = getImagen("images/semaforos/semRojoIzqda.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	semaforos = append(semaforos, f)
}

func cargarFlechas(a fyne.App) {
	for i := 1; i < 5; i++ {
		f, err := getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_dcha_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_dcha_izqda_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_fuera_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_izqda_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_recto_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_recto_dcha_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_recto_izqda_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)

		f, err = getImagen(fmt.Sprintf("images/marcas_viales/flechas/flecha_todo_%d.png", i))
		if err != nil {
			mostrarError("Error al cargar la imagen: "+err.Error(), a)
			a.Run()
			return
		}
		flechas = append(flechas, f)
	}
}

func cargarOtrasMarcas(a fyne.App) {
	f, err := getImagen("images/marcas_viales/paso.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)

	f, err = getImagen("images/marcas_viales/paso_rotado.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)

	f, err = getImagen("images/marcas_viales/discontinuaY.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)

	f, err = getImagen("images/marcas_viales/linea_doble_continuaY.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)

	f, err = getImagen("images/marcas_viales/discontinuaX.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)

	f, err = getImagen("images/marcas_viales/linea_doble_continuaX.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
	otrasMarcas = append(otrasMarcas, f)
}

func ambar(sem *canvas.Image, c *fyne.Container, permanente bool) {
	pos := getPosicionEnContainer(sem, c)
	tick := time.NewTicker(time.Second)

	imagen := c.Objects[pos].(*canvas.Image)

	if permanente {
		for {
			imagen.Image = semaforos[2]
			fyne.Do(func() { imagen.Refresh() })
			<-tick.C

			imagen.Image = semaforos[0]
			fyne.Do(func() { imagen.Refresh() })
			<-tick.C
		}
	} else {
		for i := 0; i < 10; i++ {
			imagen.Image = semaforos[2]
			fyne.Do(func() { imagen.Refresh() })
			<-tick.C

			imagen.Image = semaforos[0]
			fyne.Do(func() { imagen.Refresh() })
			<-tick.C
		}
	}

	imagen.Image = semaforos[1]
	fyne.Do(func() { imagen.Refresh() })
}

func colocarBotones(w fyne.Window, cBotones *fyne.Container, numDir int, inicializado, editar bool) {
	if !inicializado {
		for i := 0; i < numDir; i++ {
			boton := widget.NewButton("Editar "+fmt.Sprint(i+1), func() {
				menuEdicion(i+1, w)
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

func menuEdicion(dir int, w fyne.Window) {
	c := container.New(layout.NewVBoxLayout())
	var botonDirCarriles *widget.Button
	var botonSemaforos *widget.Button
	//PASO DE PEATONES
	var checkPasoPeatones *widget.Check
	pasoPeatonesActivado := false

	switch dir {
	case 1:
		for _, obj := range layoutComponentes.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(297, 360) || obj.Position() == fyne.NewPos(297, 438) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 2:
		for _, obj := range layoutComponentes.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(360, 597) || obj.Position() == fyne.NewPos(438, 597) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 3:
		for _, obj := range layoutComponentes.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(597, 360) || obj.Position() == fyne.NewPos(597, 438) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	case 4:
		for _, obj := range layoutComponentes.Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Position() == fyne.NewPos(360, 297) || obj.Position() == fyne.NewPos(438, 297) {
					pasoPeatonesActivado = true
					break
				}
			}
		}
	}

	checkPasoPeatones = widget.NewCheck("Paso de peatones", func(b bool) {
		modificarPasoPeatones(dir, b)
		pasoPeatonesActivado = b
	})

	c.Add(checkPasoPeatones)
	if pasoPeatonesActivado {
		checkPasoPeatones.SetChecked(true)
	}

	//NUM CARRILES
	numCarrilesCentro := 0
	numCarrilesFuera := 0
	labelCentro := widget.NewLabel(fmt.Sprintf("Carriles hacia el centro: %d", numCarrilesCentro))
	labelFuera := widget.NewLabel(fmt.Sprintf("Carriles hacia fuera: %d", numCarrilesFuera))

	c.Add(container.NewCenter(widget.NewLabel("Carriles (máximo 4):")))
	c.Add(labelCentro)

	sliderCarrilesHaciaCentro := widget.NewSlider(0, 4)
	sliderCarrilesHaciaFuera := widget.NewSlider(0, 4)

	sliderCarrilesHaciaCentro.Resize(fyne.NewSize(165, 50))
	sliderCarrilesHaciaCentro.OnChanged = func(i float64) {
		numCarrilesCentro = int(i)
		labelCentro.Text = fmt.Sprintf("Carriles hacia el centro: %d", numCarrilesCentro)
		labelCentro.Refresh()

		sliderCarrilesHaciaFuera.Max = float64(4 - numCarrilesCentro)
		sliderCarrilesHaciaFuera.Refresh()

		if numCarrilesFuera > 0 || numCarrilesCentro > 0 {
			botonDirCarriles.Enable()
			botonSemaforos.Enable()
		} else {
			botonDirCarriles.Disable()
			botonSemaforos.Disable()
		}

		modificarNumCarriles(dir, numCarrilesCentro, numCarrilesFuera)
		layoutComponentes.Refresh()
	}

	c.Add(sliderCarrilesHaciaCentro)
	c.Add(labelFuera)

	sliderCarrilesHaciaFuera.Resize(fyne.NewSize(165, 50))
	sliderCarrilesHaciaFuera.OnChanged = func(i float64) {
		numCarrilesFuera = int(i)
		labelFuera.Text = fmt.Sprintf("Carriles hacia fuera: %d", numCarrilesFuera)
		labelFuera.Refresh()

		sliderCarrilesHaciaCentro.Max = float64(4 - numCarrilesFuera)
		sliderCarrilesHaciaCentro.Refresh()

		if numCarrilesFuera > 0 || numCarrilesCentro > 0 {
			botonDirCarriles.Enable()
			botonSemaforos.Enable()
		} else {
			botonDirCarriles.Disable()
			botonSemaforos.Disable()
		}

		modificarNumCarriles(dir, numCarrilesCentro, numCarrilesFuera)
		layoutComponentes.Refresh()
	}

	c.Add(sliderCarrilesHaciaFuera)

	//DIR CARRILES
	botonDirCarriles = widget.NewButton("Editar dirección de los carriles", func() {
		modificarDirCarriles(dir, w)
	})
	botonDirCarriles.Disable()
	c.Add(container.NewCenter(botonDirCarriles))

	//SEMAFOROS
	botonSemaforos = widget.NewButton("Editar semáforos", func() {
		menuAddSemaforos(dir, w, pasoPeatonesActivado, numCarrilesCentro, numCarrilesFuera)
	})
	botonSemaforos.Disable()
	c.Add(container.NewCenter(botonSemaforos))

	//SI YA HAY CARRILES SE PONEN EN LOS SLIDERS Y SE HABILITAN LOS BOTONES
	if layoutsMarcas[dir-1] != nil {
		for i, obj := range layoutsMarcas[dir-1].Objects {
			if obj, ok := obj.(*canvas.Image); ok {
				if obj.Image == flechas[2] || obj.Image == flechas[10] || obj.Image == flechas[18] || obj.Image == flechas[26] {
					sliderCarrilesHaciaFuera.Value++
					numCarrilesFuera++

					labelFuera.Text = fmt.Sprintf("Carriles hacia fuera: %d", numCarrilesFuera)
					labelFuera.Refresh()

					botonDirCarriles.Enable()
					botonSemaforos.Enable()
				} else if i%2 == 0 {
					sliderCarrilesHaciaCentro.Value++
					numCarrilesCentro++

					labelCentro.Text = fmt.Sprintf("Carriles hacia el centro: %d", numCarrilesCentro)
					labelCentro.Refresh()

					botonDirCarriles.Enable()
					botonSemaforos.Enable()
				}
			}
		}
		sliderCarrilesHaciaFuera.Max = float64(4 - numCarrilesCentro)
		sliderCarrilesHaciaFuera.Refresh()

		sliderCarrilesHaciaCentro.Max = float64(4 - numCarrilesFuera)
		sliderCarrilesHaciaCentro.Refresh()
	}

	d := dialog.NewCustom(fmt.Sprintf("Editar dirección %d", dir), "Cerrar", c, w)
	d.Show()
}

func modificarPasoPeatones(dir int, b bool) {
	if b {
		pasoPeatones1 := canvas.NewImageFromImage(otrasMarcas[0])
		pasoPeatones1.FillMode = canvas.ImageFillOriginal
		pasoPeatones1.Resize(fyne.NewSize(100, 195))

		pasoPeatones2 := canvas.NewImageFromImage(otrasMarcas[0])
		pasoPeatones2.FillMode = canvas.ImageFillOriginal
		pasoPeatones2.Resize(fyne.NewSize(100, 195))

		pasoPeatonesRotado1 := canvas.NewImageFromImage(otrasMarcas[1])
		pasoPeatonesRotado1.FillMode = canvas.ImageFillOriginal
		pasoPeatonesRotado1.Resize(fyne.NewSize(195, 100))

		pasoPeatonesRotado2 := canvas.NewImageFromImage(otrasMarcas[1])
		pasoPeatonesRotado2.FillMode = canvas.ImageFillOriginal
		pasoPeatonesRotado2.Resize(fyne.NewSize(195, 100))

		switch dir {
		case 1:
			pasoPeatones1.Move(fyne.NewPos(297, 360))
			pasoPeatones2.Move(fyne.NewPos(297, 438))

			layoutComponentes.Add(pasoPeatones1)
			layoutComponentes.Add(pasoPeatones2)
		case 2:
			pasoPeatonesRotado1.Move(fyne.NewPos(360, 597))
			pasoPeatonesRotado2.Move(fyne.NewPos(438, 597))

			layoutComponentes.Add(pasoPeatonesRotado1)
			layoutComponentes.Add(pasoPeatonesRotado2)
		case 3:
			pasoPeatones1.Move(fyne.NewPos(597, 360))
			pasoPeatones2.Move(fyne.NewPos(597, 438))

			layoutComponentes.Add(pasoPeatones1)
			layoutComponentes.Add(pasoPeatones2)
		case 4:
			pasoPeatonesRotado1.Move(fyne.NewPos(360, 297))
			pasoPeatonesRotado2.Move(fyne.NewPos(438, 297))

			layoutComponentes.Add(pasoPeatonesRotado1)
			layoutComponentes.Add(pasoPeatonesRotado2)
		}

		layoutComponentes.Refresh()
	} else {
		var objetosAEliminar []fyne.CanvasObject
		for _, obj := range layoutComponentes.Objects {
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
			layoutComponentes.Remove(obj)
		}

		layoutComponentes.Refresh()
	}
}

func modificarNumCarriles(dir int, numCarrilesDentro, numCarrilesFuera int) {
	layoutsDirs := make([]*fyne.Container, 4)

	//BORRAR LAS FLECHAS ANTERIORES
	switch dir {
	case 1:
		for _, c := range layoutComponentes.Objects {
			if c != nil && c.Position() == fyne.NewPos(70, 397) {
				layoutComponentes.Remove(c)
				layoutsMarcas[dir-1] = nil
			}
		}
	case 2:
		for _, c := range layoutComponentes.Objects {
			if c != nil && c.Position() == fyne.NewPos(397, 710) {
				layoutComponentes.Remove(c)
				layoutsMarcas[dir-1] = nil
			}
		}
	case 3:
		for _, c := range layoutComponentes.Objects {
			if c != nil && c.Position() == fyne.NewPos(710, 397) {
				layoutComponentes.Remove(c)
				layoutsMarcas[dir-1] = nil
			}
		}
	case 4:
		for _, c := range layoutComponentes.Objects {
			if c != nil && c.Position() == fyne.NewPos(397, 70) {
				layoutComponentes.Remove(c)
				layoutsMarcas[dir-1] = nil
			}
		}
	}

	//AJUSTAR TAMAÑO DE FLECHAS CON PROPORCION 82x231 ALTURA MAX 198
	var sizeFlechas fyne.Size
	switch numCarrilesDentro + numCarrilesFuera {
	case 0:
		return
	case 1, 2:
		if dir%2 == 1 {
			sizeFlechas = fyne.NewSize(198, 70)
		} else {
			sizeFlechas = fyne.NewSize(70, 198)
		}
	case 3:
		if dir%2 == 1 {
			sizeFlechas = fyne.NewSize(124, 44)
		} else {
			sizeFlechas = fyne.NewSize(44, 124)
		}
	case 4:
		if dir%2 == 1 {
			sizeFlechas = fyne.NewSize(73, 26)
		} else {
			sizeFlechas = fyne.NewSize(26, 73)
		}
	}

	//CREAR FLECHAS
	var c *fyne.Container
	switch dir {
	case 1:
		c = container.New(&layouts.CarrilesVerticales{})
		c.Move(fyne.NewPos(70, 397))
		c.Resize(fyne.NewSize(198, 198))

		for i := 1; i <= numCarrilesFuera; i++ {
			flecha := canvas.NewImageFromImage(flechas[2+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesFuera {
				linea := canvas.NewImageFromImage(otrasMarcas[4])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(198, 30))
			}
		}

		if numCarrilesFuera > 0 && numCarrilesDentro > 0 {
			continua := canvas.NewImageFromImage(otrasMarcas[5])
			continua.FillMode = canvas.ImageFillStretch
			c.Add(continua)
			continua.Resize(fyne.NewSize(198, 30))
		}

		for i := 1; i <= numCarrilesDentro; i++ {
			flecha := canvas.NewImageFromImage(flechas[4+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesDentro {
				linea := canvas.NewImageFromImage(otrasMarcas[4])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(198, 30))
			}
		}

		layoutsDirs[dir-1] = c
	case 2:
		c = container.New(&layouts.CarrilesHorizontales{})
		c.Move(fyne.NewPos(397, 710))
		c.Resize(fyne.NewSize(198, 198))

		for i := 1; i <= numCarrilesFuera; i++ {
			flecha := canvas.NewImageFromImage(flechas[2+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesFuera {
				linea := canvas.NewImageFromImage(otrasMarcas[2])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(30, 198))
			}
		}

		if numCarrilesFuera > 0 && numCarrilesDentro > 0 {
			continua := canvas.NewImageFromImage(otrasMarcas[3])
			continua.FillMode = canvas.ImageFillStretch
			c.Add(continua)
			continua.Resize(fyne.NewSize(30, 198))
		}

		for i := 1; i <= numCarrilesDentro; i++ {
			flecha := canvas.NewImageFromImage(flechas[4+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesDentro {
				linea := canvas.NewImageFromImage(otrasMarcas[2])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(30, 198))
			}
		}

		layoutsDirs[dir-1] = c
	case 3:
		c = container.New(&layouts.CarrilesVerticales{})
		c.Move(fyne.NewPos(710, 397))
		c.Resize(fyne.NewSize(198, 198))

		for i := 1; i <= numCarrilesDentro; i++ {
			flecha := canvas.NewImageFromImage(flechas[4+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesDentro {
				linea := canvas.NewImageFromImage(otrasMarcas[4])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(198, 30))
			}
		}

		if numCarrilesFuera > 0 && numCarrilesDentro > 0 {
			continua := canvas.NewImageFromImage(otrasMarcas[5])
			continua.FillMode = canvas.ImageFillStretch
			c.Add(continua)
			continua.Resize(fyne.NewSize(198, 30))
		}

		for i := 1; i <= numCarrilesFuera; i++ {
			flecha := canvas.NewImageFromImage(flechas[2+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesFuera {
				linea := canvas.NewImageFromImage(otrasMarcas[4])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(198, 30))
			}
		}

		layoutsDirs[dir-1] = c
	case 4:
		c = container.New(&layouts.CarrilesHorizontales{})
		c.Move(fyne.NewPos(397, 70))
		c.Resize(fyne.NewSize(198, 198))

		for i := 1; i <= numCarrilesDentro; i++ {
			flecha := canvas.NewImageFromImage(flechas[4+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesDentro {
				linea := canvas.NewImageFromImage(otrasMarcas[2])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(30, 198))
			}
		}

		if numCarrilesFuera > 0 && numCarrilesDentro > 0 {
			continua := canvas.NewImageFromImage(otrasMarcas[3])
			continua.FillMode = canvas.ImageFillStretch
			c.Add(continua)
			continua.Resize(fyne.NewSize(30, 198))
		}

		for i := 1; i <= numCarrilesFuera; i++ {
			flecha := canvas.NewImageFromImage(flechas[2+(8*(dir-1))])
			flecha.FillMode = canvas.ImageFillStretch
			c.Add(flecha)
			flecha.Resize(sizeFlechas)

			if i < numCarrilesFuera {
				linea := canvas.NewImageFromImage(otrasMarcas[2])
				linea.FillMode = canvas.ImageFillStretch
				c.Add(linea)
				linea.Resize(fyne.NewSize(30, 198))
			}
		}

		layoutsDirs[dir-1] = c
	}

	if c != nil {
		layoutComponentes.Add(c)
		layoutsMarcas[dir-1] = c
	}
}

func modificarDirCarriles(dir int, w fyne.Window) {
	c := container.New(layout.NewGridLayout(2))

	//RECORRERSE LOS CARRILES Y COLOCAR BOTONES
	for _, obj := range layoutsMarcas[dir-1].Objects {
		textoBtn := ""

		if obj, ok := obj.(*canvas.Image); ok {
			switch getPosicionFlecha(obj.Image, flechas) % 8 {
			case 0: //dcha
				textoBtn = "Derecha"
			case 1: //dcha_izqda
				textoBtn = "Derecha-Izquierda"
			// case 2 es hacia fuera, por lo que no se puede editar
			case 3: //izqda
				textoBtn = "Izquierda"
			case 4: //recto (por defecto)
				textoBtn = "Recto"
			case 5: //recto_dcha
				textoBtn = "Recto-Derecha"
			case 6: //recto_izqda
				textoBtn = "Recto-Izquierda"
			case 7: //todo
				textoBtn = "Todo"
			}

			if textoBtn != "" { //CREAR BOTON
				var btn *widget.Button
				btn = widget.NewButton(textoBtn, func() {
					selectDireccion := widget.NewSelect([]string{"Recto", "Derecha", "Izquierda", "Recto-Derecha", "Recto-Izquierda", "Derecha-Izquierda", "Todas direcciones"},
						func(s string) {
							switch s {
							case "Recto":
								obj.Image = flechas[4+(8*(dir-1))]
							case "Derecha":
								obj.Image = flechas[0+(8*(dir-1))]
							case "Izquierda":
								obj.Image = flechas[3+(8*(dir-1))]
							case "Recto-Derecha":
								obj.Image = flechas[5+(8*(dir-1))]
							case "Recto-Izquierda":
								obj.Image = flechas[6+(8*(dir-1))]
							case "Derecha-Izquierda":
								obj.Image = flechas[1+(8*(dir-1))]
							case "Todas direcciones":
								obj.Image = flechas[7+(8*(dir-1))]
							}
							obj.Refresh()

							btn.Text = s
							btn.Refresh()
						})
					selectDireccion.SetSelected(btn.Text)

					dialogDireccion := dialog.NewCustom("Seleccionar dirección", "Cerrar", selectDireccion, w)
					dialogDireccion.Show()
				})
				c.Add(btn)
			}
			textoBtn = ""
		}
	}

	d := dialog.NewCustom(fmt.Sprintf("Editar carriles %d", dir), "Cerrar", c, w)
	d.Show()
}

func menuAddSemaforos(dir int, w fyne.Window, peatones bool, numCarrilesCentro, numCarrilesFuera int) {
	cEspacio := container.New(layout.NewGridLayout(4))
	cBotones := container.New(layout.NewGridLayout(4))
	var cDerecha *fyne.Container
	var cIzquierda *fyne.Container
	existen := false

	//COMPROBAR EXISTENTES
	for _, l := range layoutComponentes.Objects {
		switch dir {
		case 1:
			if l.Position() == fyne.NewPos(163, 255) {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
					existen = true
				}
			} else if l.Position() == fyne.NewPos(163, 647) {
				if l, ok := l.(*fyne.Container); ok {
					cDerecha = l
					existen = true
				}
			}
		case 2:
			if l.Position() == fyne.NewPos(255, 730) {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
					existen = true
				}
			} else if l.Position() == fyne.NewPos(647, 730) {
				if l, ok := l.(*fyne.Container); ok {
					cDerecha = l
					existen = true
				}
			}
		case 3:
			if l.Position() == fyne.NewPos(730, 647) {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
					existen = true
				}
			} else if l.Position() == fyne.NewPos(730, 255) {
				if l, ok := l.(*fyne.Container); ok {
					cDerecha = l
					existen = true
				}
			}
		case 4:
			if l.Position() == fyne.NewPos(647, 163) {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
					existen = true
				}
			} else if l.Position() == fyne.NewPos(255, 163) {
				if l, ok := l.(*fyne.Container); ok {
					cDerecha = l
					existen = true
				}
			}
		}
	}

	if !existen {
		//COLOCAR LAYOUTS
		cDerecha = container.New(&layouts.Semaforos{})
		cIzquierda = container.New(&layouts.Semaforos{})

		sizeLayout := fyne.NewSize(90, 100)

		switch dir {
		case 1:
			cIzquierda.Move(fyne.NewPos(163, 255))
			cDerecha.Move(fyne.NewPos(163, 647))
		case 2:
			cDerecha.Move(fyne.NewPos(647, 730))
			cIzquierda.Move(fyne.NewPos(255, 730))
		case 3:
			cIzquierda.Move(fyne.NewPos(730, 647))
			cDerecha.Move(fyne.NewPos(730, 255))
		case 4:
			cDerecha.Move(fyne.NewPos(255, 163))
			cIzquierda.Move(fyne.NewPos(647, 163))
		}

		cDerecha.Resize(sizeLayout)
		cIzquierda.Resize(sizeLayout)
	}

	//BOTON DE CREACION DE SEMAFOROS
	var btnAddSemaforoFuera, btnAddSemaforoDentro, btnAddSemaforoPeatones *widget.Button

	if numCarrilesFuera != 0 { //HAY CARRILES HACIA FUERA POR LO QUE SE AÑADE BOTON DE SEMAFORO SALIENTE
		//COMPROBAR SI YA SE HA AÑADIDO UN SEMAFORO SALIENTE
		haySaliente := false
		if len(cIzquierda.Objects) == 1 {
			haySaliente = true
		}

		btnAddSemaforoFuera = widget.NewButton("Añadir semáforo\npara sentido saliente", func() {
			//crear semaforo
			sem := canvas.NewImageFromImage(semaforos[0])
			sem.FillMode = canvas.ImageFillOriginal
			sem.Resize(semaforoSize)

			cIzquierda.Add(sem)

			if numCarrilesCentro == 0 {
				//añadir en el otro lado también al haber un único sentido
				cDerecha.Add(sem)
			}
			//crear boton
			var b *widget.Button
			b = widget.NewButton("Semáforo saliente", func() {
				menuSecuenciaSemaforos(0, w, false, sem, getPosicionEnArray(sem, cIzquierda.Objects), func() {
					cBotones.Remove(b)
					cBotones.Refresh()
					btnAddSemaforoFuera.Enable()
				})
			})

			botonesSemaforos[dir] = append(botonesSemaforos[dir], b)
			cBotones.Add(b)
			btnAddSemaforoFuera.Disable() //solo puede haber 1 saliente
		})
		cBotones.Add(btnAddSemaforoFuera)

		if haySaliente {
			btnAddSemaforoFuera.Disable()
			b := widget.NewButton("Semáforo saliente", func() {
				if sem, ok := cIzquierda.Objects[0].(*canvas.Image); ok {
					var b *widget.Button
					menuSecuenciaSemaforos(0, w, false, sem, getPosicionEnArray(sem, cIzquierda.Objects), func() {
						cBotones.Remove(b)
						cBotones.Refresh()
						btnAddSemaforoFuera.Enable()
					})
				}
			})
			cBotones.Add(b)
		}
	}

	if numCarrilesCentro != 0 { //HAY CARRILES HACIA DENTRO POR LO QUE SE AÑADE BOTON DE SEMAFORO ENTRANTE
		hayEntrante := false

		if len(cDerecha.Objects) > 0 { //comprobar que haya semaforos entrantes
			hayEntrante = true
		}

		var maxDirs int
		if numDirecciones == 3 {
			maxDirs = 2
		} else {
			maxDirs = 3
		}

		btnAddSemaforoDentro = widget.NewButton("Añadir semáforo\npara sentido entrante", func() {
			//crear semaforo
			sem := canvas.NewImageFromImage(semaforos[0])
			sem.FillMode = canvas.ImageFillOriginal
			sem.Resize(semaforoSize)

			cDerecha.Add(sem)

			if numCarrilesFuera == 0 {
				//añadir en el otro lado también al haber un único sentido
				cIzquierda.Add(sem)
			}

			//crear boton
			b := widget.NewButton("Semáforo entrante", func() {
				var b *widget.Button
				menuSecuenciaSemaforos(dir, w, true, sem, getPosicionEnArray(sem, cDerecha.Objects), func() {
					cBotones.Remove(b)
					cBotones.Refresh()
					btnAddSemaforoDentro.Enable()
				})
			})

			botonesSemaforos[dir] = append(botonesSemaforos[dir], b)
			cBotones.Add(b)

			i := 0

			for _, obj := range cBotones.Objects {
				if obj, ok := obj.(*widget.Button); ok {
					if obj.Text == "Semáforo entrante" {
						i++
						if i == maxDirs { //puede haber 1 semaforo para cada direccion posible maximo
							btnAddSemaforoDentro.Disable()
							return
						}
					}
				}
			}
		})
		cBotones.Add(btnAddSemaforoDentro)

		if hayEntrante {
			for i, obj := range cDerecha.Objects {
				if i == maxDirs-1 {
					btnAddSemaforoDentro.Disable()
				}

				b := widget.NewButton("Semáforo entrante", func() {
					if sem, ok := obj.(*canvas.Image); ok {
						var b *widget.Button
						menuSecuenciaSemaforos(dir, w, true, sem, getPosicionEnArray(sem, cDerecha.Objects), func() {
							cBotones.Remove(b)
							cBotones.Refresh()
							btnAddSemaforoDentro.Enable()
						})
					}
				})
				cBotones.Add(b)
			}
		}
	}

	if peatones {
		btnAddSemaforoPeatones = widget.NewButton("Añadir semáforo\npara peatones", func() {
			b := widget.NewButton("Semáforo peatonal", func() {
				//TODO
			})

			botonesSemaforos[dir] = append(botonesSemaforos[dir], b)
			cBotones.Add(b)
			btnAddSemaforoPeatones.Disable() //solo hay 1 pareja de semaforos peatonales
		})
		cBotones.Add(btnAddSemaforoPeatones)
	}

	d := dialog.NewCustom(fmt.Sprintf("Editar semáforos %d", dir), "Cerrar", container.NewVBox(cEspacio, cBotones), w)
	d.Show()

	if cIzquierda != nil {
		layoutComponentes.Add(cIzquierda)
		layoutsSemaforos[dir-1] = cIzquierda
	}

	if cDerecha != nil {
		layoutComponentes.Add(cDerecha)
		layoutsSemaforos[dir+3] = cDerecha
	}
}

type Secuencia struct {
	Semaforo  *canvas.Image   `json:"sem"`
	Direccion int             `json:"dir"`
	Colores   []string        `json:"colores"`
	Tiempos   []time.Duration `json:"tiempos"`
	Posicion  int             `json:"pos"`
}

func menuSecuenciaSemaforos(dir int, w fyne.Window, entrante bool, sem *canvas.Image, pos int, onDelete func()) { //onDelete borra el botón al borrar el semáforo
	c := container.NewVBox()
	var opciones []string
	cSecuencia := container.New(layout.NewGridLayout(3))

	//si es entrante puede cambiar la dirección
	if entrante {
		labelDir := widget.NewLabel("Dirección del semáforo")
		c.Add(container.NewCenter(labelDir))

		if numDirecciones == 4 {
			opciones = []string{"General", "Derecha", "Frente", "Izquierda"}
		} else {
			switch dir {
			case 1:
				opciones = []string{"General", "Frente", "Derecha"}
			case 2:
				opciones = []string{"General", "Derecha", "Izquierda"}
			case 3:
				opciones = []string{"General", "Frente", "Izquierda"}
			}
		}

		comboBox := widget.NewSelect(opciones, func(value string) {
			switch value {
			case "General":
				sem.Image = semaforos[3]
				sem.Refresh()
			case "Derecha":
				sem.Image = semaforos[6]
				sem.Refresh()
			case "Frente":
				sem.Image = semaforos[9]
				sem.Refresh()
			case "Izquierda":
				sem.Image = semaforos[12]
				sem.Refresh()
			}
		})

		//PONER LA DIRECCIÓN SI YA HA SIDO ELEGIDA
		switch sem.Image {
		case semaforos[3]:
			comboBox.SetSelected("General")
		case semaforos[6]:
			comboBox.SetSelected("Derecha")
		case semaforos[9]:
			comboBox.SetSelected("Frente")
		case semaforos[12]:
			comboBox.SetSelected("Izquierda")
		default:
			comboBox.SetSelectedIndex(0)
		}

		c.Add(comboBox)
	}

	labelSec := widget.NewLabel("Secuencia")
	c.Add(container.NewCenter(labelSec))

	//EDITAR SECUENCIA
	var d *dialog.CustomDialog
	var coloresUsados []string

	btnSecuencia := widget.NewButton("Añadir fase", func() {
		//ELEGIR COLOR
		var coloresRecortado []string
		for _, color := range colores {
			if !slices.Contains(coloresUsados, color) {
				coloresRecortado = append(coloresRecortado, color)
			}
		}

		comboBoxColor := widget.NewSelect(coloresRecortado, func(value string) {
			coloresUsados, coloresRecortado = recargarColores(cSecuencia)
		})

		comboBoxColor.PlaceHolder = "Elegir color"
		cSecuencia.Add(comboBoxColor)

		//ELEGIR TIEMPO
		inputTiempo := widget.NewEntry()
		inputTiempo.SetPlaceHolder("Segundos")
		inputTiempo.OnChanged = func(str string) {
			if i, err := strconv.Atoi(str); err != nil || i >= 100 || i <= 0 { //comprobar si es numerico, no negativo y menor a 100 sec
				inputTiempo.Text = ""
			}
		}

		cSecuencia.Add(inputTiempo)

		//BORRAR FASE
		var btnBorrar *widget.Button
		btnBorrar = widget.NewButton("Borrar", func() {
			cSecuencia.Remove(comboBoxColor)
			cSecuencia.Remove(inputTiempo)
			cSecuencia.Remove(btnBorrar)

			coloresUsados, coloresRecortado = recargarColores(cSecuencia)
		})

		cSecuencia.Add(btnBorrar)
	})

	c.Add(btnSecuencia)
	c.Add(cSecuencia)

	//COMPROBAR FASES EXISTENTES Y COLOCARLAS
	if len(secuencias[dir]) > 0 {
		for _, sec := range secuencias[dir] {
			if sec.Posicion == pos {
				coloresUsados = colocarFases(coloresUsados, sec, cSecuencia)
				break
			}
		}
	}

	cBotones := container.New(layout.NewGridLayout(5))

	btnCopiar := widget.NewButton("Copiar fases", func() {
		fasesCopiada = obtenerSecuencia(cSecuencia, sem, dir, pos, w)
	})
	cBotones.Add(btnCopiar)

	btnPegar := widget.NewButton("Pegar fases", func() {
		coloresUsados = colocarFases(coloresUsados, fasesCopiada, cSecuencia)
	})
	cBotones.Add(btnPegar)

	btnBorrar := widget.NewButton("Borrar semáforo", func() {
		dialog.ShowConfirm("Borrar semáforo", "¿Seguro que quieres borrar este semáforo?", func(b bool) {
			for dirIdx := range secuencias {
				var nuevasSecuencias []Secuencia
				for _, sec := range secuencias[dirIdx] {
					if sec.Semaforo != sem {
						nuevasSecuencias = append(nuevasSecuencias, sec)
					}
				}
				secuencias[dirIdx] = nuevasSecuencias
			}

			for dirIdx := range botonesSemaforos {
				var nuevosBotones []*widget.Button
				nuevosBotones = append(nuevosBotones, botonesSemaforos[dirIdx]...)
				botonesSemaforos[dirIdx] = nuevosBotones
			}

			for _, cont := range layoutsSemaforos {
				if cont == nil {
					continue
				}
				var nuevosObjs []fyne.CanvasObject
				for _, obj := range cont.Objects {
					if img, ok := obj.(*canvas.Image); ok && img == sem {
						continue
					}
					nuevosObjs = append(nuevosObjs, obj)
				}
				cont.Objects = nuevosObjs
				cont.Refresh()
			}

			var nuevosObjs []fyne.CanvasObject
			for _, obj := range layoutComponentes.Objects {
				if img, ok := obj.(*canvas.Image); ok && img == sem {
					continue
				}
				nuevosObjs = append(nuevosObjs, obj)
			}
			layoutComponentes.Objects = nuevosObjs
			layoutComponentes.Refresh()

			d.Hide()
			onDelete()
		}, w)
	})
	cBotones.Add(btnBorrar)

	btnGuardar := widget.NewButton("Guardar", func() {
		s := obtenerSecuencia(cSecuencia, sem, dir, pos, w)

		//SE ALMACENAN LAS SECUENCIAS, SI YA ESTABAN ALMACENADAS SE SOBREESCRIBEN
		existe := false
		for i, sec := range secuencias[dir] {
			if sec.Posicion == s.Posicion {
				secuencias[dir][i] = s
				existe = true
				break
			}
		}

		if !existe {
			secuencias[dir] = append(secuencias[dir], s)
		}
	})
	cBotones.Add(btnGuardar)

	btnVolver := widget.NewButton("Volver", func() {
		d.Hide()
	})
	cBotones.Add(btnVolver)

	c.Add(cBotones)

	d = dialog.NewCustomWithoutButtons("Secuencia de semáforo", c, w)
	d.Show()
}

func obtenerSecuencia(cSecuencia *fyne.Container, sem *canvas.Image, dir int, pos int, w fyne.Window) Secuencia {
	var colores []string
	var tiempos []time.Duration

	for _, fase := range cSecuencia.Objects {
		if comboBox, ok := fase.(*widget.Select); ok {
			colores = append(colores, comboBox.Selected)
		} else if entry, ok := fase.(*widget.Entry); ok {
			if entry.Text == "" {
				entry.Text = "0"
			}
			if i, err := strconv.Atoi(entry.Text); err == nil {
				tiempos = append(tiempos, time.Second*time.Duration(i))
			} else {
				mostrarError(err.Error(), fyne.CurrentApp(), w)
			}
		}
	}

	s := Secuencia{
		sem,
		dir,
		colores,
		tiempos,
		pos,
	}

	return s
}

func colocarFases(coloresUsados []string, sec Secuencia, cSecuencia *fyne.Container) []string {
	//fmt.Println("Colocando: ", sec)
	for i, color := range sec.Colores {
		//SIMILAR AL FUNCIONAMIENTO DE "AÑADIR FASE" PERO COMPLETANDO DATOS
		var coloresRecortado []string
		for _, color := range colores {
			if !slices.Contains(coloresUsados, color) {
				coloresRecortado = append(coloresRecortado, color)
			}
		}

		comboBoxColor := widget.NewSelect(coloresRecortado, func(value string) {
			coloresUsados, _ = recargarColores(cSecuencia)
		})
		comboBoxColor.Selected = color
		cSecuencia.Add(comboBoxColor)

		inputTiempo := widget.NewEntry()
		inputTiempo.SetPlaceHolder("Segundos")
		inputTiempo.OnChanged = func(str string) {
			if i, err := strconv.Atoi(str); err != nil || i >= 100 || i <= 0 { //comprobar si es numerico, no negativo y menor a 100 sec
				inputTiempo.Text = ""
			}
		}
		inputTiempo.Text = strconv.Itoa(int(sec.Tiempos[i].Seconds()))
		cSecuencia.Add(inputTiempo)

		var btnBorrar *widget.Button
		btnBorrar = widget.NewButton("Borrar", func() {
			cSecuencia.Remove(comboBoxColor)
			cSecuencia.Remove(inputTiempo)
			cSecuencia.Remove(btnBorrar)

			coloresUsados, coloresRecortado = recargarColores(cSecuencia)
		})

		cSecuencia.Add(btnBorrar)
	}

	return coloresUsados
}

func recargarColores(cSecuencia *fyne.Container) ([]string, []string) {
	coloresUsados := []string{}

	//GUARDAR COLORES USADOS
	for _, obj := range cSecuencia.Objects {
		if comboBox, ok := obj.(*widget.Select); ok && comboBox.Selected != "" {
			coloresUsados = append(coloresUsados, comboBox.Selected)
		}
	}

	//ACTUALIZAR COLORES EN LOS COMBOBOX
	for _, obj := range cSecuencia.Objects {
		if comboBox, ok := obj.(*widget.Select); ok {
			opciones := []string{}

			if comboBox.Selected != "" {
				opciones = append(opciones, comboBox.Selected)
			}

			for _, color := range colores {
				if color == comboBox.Selected {
					continue
				}

				if !slices.Contains(coloresUsados, color) {
					opciones = append(opciones, color)
				}
			}
			comboBox.Options = opciones
			comboBox.Refresh()
		}
	}

	//GUARDAR COLORES LIBRES
	coloresRecortado := []string{}
	for _, color := range colores {
		enUso := false
		for _, usado := range coloresUsados {
			if usado == color {
				enUso = true
				break
			}
		}
		if !enUso {
			coloresRecortado = append(coloresRecortado, color)
		}
	}

	return coloresUsados, coloresRecortado
}

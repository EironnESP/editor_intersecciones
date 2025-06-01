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
	"strings"
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
var id = -1
var nombre = ""

// imágenes
var flechas []image.Image
var semaforos []image.Image
var otrasMarcas []image.Image
var fondos = make([]image.Image, 2)

// contenedores
var layoutsMarcas = make([]*fyne.Container, 4)
var layoutsSemaforos = make([]*fyne.Container, 8)
var layoutComponentes *fyne.Container
var layoutBotonesEditar *fyne.Container

// posiciones
var posSemIzqda = []fyne.Position{
	fyne.NewPos(163, 255), // dir 1
	fyne.NewPos(255, 730), // dir 2
	fyne.NewPos(730, 647), // dir 3
	fyne.NewPos(647, 163), // dir 4
}
var posSemDcha = []fyne.Position{
	fyne.NewPos(163, 647), // dir 1
	fyne.NewPos(647, 730), // dir 2
	fyne.NewPos(730, 255), // dir 3
	fyne.NewPos(255, 163), // dir 4
}

// ejecución
var tickerEjecucion *time.Ticker
var tickBroadcast = make(chan struct{})
var pausado = false
var chanParar = make(chan struct{})

// otros
var semaforoSize = fyne.NewSize(28, 100)
var numDirecciones = 0
var secuencias = make([][]Secuencia, 5)
var botonesSemaforos = make([][]*widget.Button, 5)
var colores = []string{"Verde", "Ámbar", "Ámbar (parpadeo)", "Rojo"}
var fasesCopiada Secuencia
var pasosPeatones = make([]bool, 4)

// 0 centro, 1 fuera
var numCarrilesPrevios = make([]int, 2)

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

	//FONDO
	cargarFondos(a)
	image := canvas.NewImageFromImage(fondos[1])
	image.FillMode = canvas.ImageFillOriginal

	layoutBotonesEditar = container.NewWithoutLayout()
	layoutBotonesEditar.Resize(fyne.NewSize(994, 993))

	layoutComponentes = container.NewWithoutLayout()
	layoutComponentes.Resize(fyne.NewSize(994, 993))

	fondo := container.NewCenter(container.NewStack(image, layoutComponentes, layoutBotonesEditar))

	//BARRAS DE HERRAMIENTAS
	barraHerramientasEdicion := widget.NewToolbar(
		widget.NewToolbarAction(theme.LogoutIcon(), func() {
			salir(w)
		}),
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() {
			entryNombre := widget.NewEntry()
			entryNombre.SetText(nombre)
			entryNombre.Validator = func(s string) error {
				if len(s) == 0 {
					return fmt.Errorf("El nombre no puede estar vacío")
				} else {
					return nil
				}
			}

			fi := []*widget.FormItem{{Text: "Nombre del diseño:", Widget: entryNombre}}

			dialogNombre := dialog.NewForm("Guardar", "Guardar diseño", "Cancelar", fi, func(b bool) {
				if b {
					nombre = entryNombre.Text
					guardarBBDD(entryNombre.Text)
				}
			}, w)
			dialogNombre.Show()
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
			dialogNoImplementado := dialog.NewCustom("Cambiar orientación", "Cerrar", widget.NewLabel("La funcionalidad para cambiar de orientación el plano aún no está implementada"), w)
			dialogNoImplementado.Show()
		}),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			mostrarAyuda(w)
		}),
	)

	barraHerramientasEjecucion := widget.NewToolbar(
		widget.NewToolbarAction(theme.LogoutIcon(), func() {
			salir(w)
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() {
			pausado = false
		}),
		widget.NewToolbarAction(theme.MediaPauseIcon(), func() {
			pausado = true
		}),
		widget.NewToolbarAction(theme.MediaReplayIcon(), func() {
			pararEjecucion()
			go ejecucion(layoutComponentes)
		}),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			mostrarAyuda(w)
		}),
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
			tickerEjecucion = time.NewTicker(time.Second)
			go ejecucion(layoutComponentes)
		} else {
			pararEjecucion()
		}
	}

	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)
	w.SetCloseIntercept(func() {
		salir(w)
	})
	w.Show()

	//DIALOG NUEVO DISEÑO O EXISTENTE
	var c *fyne.Container
	var d *dialog.CustomDialog
	var dDir *dialog.CustomDialog

	botonNuevo := widget.NewButton("Nuevo diseño", func() {
		boton3 := widget.NewButton("3 direcciones", func() {
			numDirecciones = 3
			image.Image = fondos[0]
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
		abrirBBDD(c, w, d, image)
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
	if _, err := os.Stat(home + "/.intersecciones/bbdd.db"); os.IsNotExist(err) { //futuro: comprobacion version actualizar estructura bbdd
		err := os.MkdirAll(home+"/.intersecciones/", 0755)
		if err != nil {
			return err
		}
		_, err = os.Create(home + "/.intersecciones/bbdd.db")
		if err != nil {
			return err
		}

		db, err := sql.Open("sqlite3", home+"/.intersecciones/bbdd.db")

		if err != nil {
			return err
		}

		defer db.Close()

		s := `
		DROP TABLE IF EXISTS Intersecciones;
		CREATE TABLE Intersecciones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nombre TEXT,
			num_direcciones INTEGER
		);

		DROP TABLE IF EXISTS Direcciones;
		CREATE TABLE Direcciones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			interseccion_id INTEGER,
			direccion INTEGER, -- 1 = izqda, 2 = abajo, 3 = dcha, 4 = arriba
			tiene_paso_peatones INTEGER, 
			FOREIGN KEY(interseccion_id) REFERENCES Intersecciones(id)
			ON DELETE CASCADE
		);

		DROP TABLE IF EXISTS Carriles;
		CREATE TABLE Carriles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			interseccion_id INTEGER,
			direccion_id INTEGER,
			sentido INTEGER, -- 0 = centro, 1 = fuera
			tipo_flecha INTEGER, -- 0 dcha 1 dcha_izqda, 2 fuera, 3 izqda, 4 recto, 5 recto_dcha, 6 recto_izqda, 7 todo
			FOREIGN KEY(interseccion_id) REFERENCES Intersecciones(id)
			ON DELETE CASCADE,
			FOREIGN KEY(direccion_id) REFERENCES Direcciones(id)
			ON DELETE CASCADE
		);

		DROP TABLE IF EXISTS Semaforos;
		CREATE TABLE Semaforos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			interseccion_id INTEGER,
			direccion_id INTEGER,
			colores TEXT, -- separado por ,
			segundos TEXT, -- separado por ,
			posicion INTEGER, 
			saliente INTEGER, -- 0 entrante, 1 saliente
			dir_flecha INTEGER, -- 0 general, 1 derecha, 2 izquierda, 3 frente
			FOREIGN KEY(interseccion_id) REFERENCES Intersecciones(id)
			ON DELETE CASCADE,
			FOREIGN KEY(direccion_id) REFERENCES Direcciones(id)
			ON DELETE CASCADE
		);        
		`
		_, err = db.Exec(s)

		if err != nil {
			return err
		}

		return nil
	} else {
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

func cargarFondos(a fyne.App) {
	var err error
	fondos[0], err = getImagen("images/cruces/cruce3_vacio.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}

	fondos[1], err = getImagen("images/cruces/cruce_vacio.png")
	if err != nil {
		mostrarError("Error al cargar la imagen: "+err.Error(), a)
		a.Run()
		return
	}
}

// Ámbar parpadeante
func ambar(sem *canvas.Image, dir, segundos int) {
	tick := time.NewTicker(time.Second)

	for i := 0; i < segundos/2; i++ {
		sem.Image = semaforos[(3*dir)+2]
		fyne.Do(func() { sem.Refresh() })
		<-tick.C

		sem.Image = semaforos[0]
		fyne.Do(func() { sem.Refresh() })
		<-tick.C
	}

	sem.Image = semaforos[1]
	fyne.Do(func() { sem.Refresh() })
}

// Colocación de los botones de menús en cada dirección
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
		for _, boton := range cBotones.Objects {
			boton := boton.(*widget.Button)
			boton.Hidden = !editar
			boton.Refresh()
		}
	}
}

// Menú para utilizar el resto de funcionalidades
func menuEdicion(dir int, w fyne.Window) {
	c := container.New(layout.NewVBoxLayout())
	var botonDirCarriles *widget.Button
	var botonSemaforos *widget.Button
	var cDerecha *fyne.Container
	var cIzquierda *fyne.Container
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
		pasosPeatones[dir-1] = b
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

		//BORRAR SEMAFORO ENTRANTE SI LO HAY Y YA NO HAY CARRILES ENTRANTES
		if numCarrilesCentro == 0 && cDerecha != nil && len(cDerecha.Objects) > 0 {
			cDerecha.Objects = nil
			cDerecha.Refresh()
			//BORRAR AL OTRO LADO SI SOLO HABÍA 1 DIRECCIÓN
			if numCarrilesFuera == 0 && cIzquierda != nil && len(cIzquierda.Objects) > 0 {
				cIzquierda.Objects = nil
				cIzquierda.Refresh()
			} else if len(cIzquierda.Objects) > 0 { //SI HAY SEMAFOROS EN SENTIDO CONTRARIO SE DUPLICAN
				for _, sem := range cIzquierda.Objects {
					if sem, ok := sem.(*canvas.Image); ok {
						cDerecha.Add(sem)
					}
				}
			}
		} else {
			layoutComponentes.Refresh()
		}

		//BORRAR SEMAFORO DEL OTRO LADO AL AÑADIR CARRILES EN ESTE SENTIDO
		if numCarrilesPrevios[0] == 0 && cDerecha != nil && len(cDerecha.Objects) > 0 {
			cDerecha.Objects = nil
			cDerecha.Refresh()
		}

		numCarrilesPrevios[0] = numCarrilesCentro
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

		//BORRAR SEMAFORO SALIENTE SI LO HAY Y YA NO HAY CARRILES SALIENTES
		if numCarrilesFuera == 0 && cIzquierda != nil && len(cIzquierda.Objects) > 0 {
			cIzquierda.Objects = nil
			cIzquierda.Refresh()
			//BORRAR AL OTRO LADO SI SOLO HABÍA 1 DIRECCIÓN
			if numCarrilesCentro == 0 && cDerecha != nil && len(cDerecha.Objects) > 0 {
				cDerecha.Objects = nil
				cDerecha.Refresh()
			} else if len(cDerecha.Objects) > 0 { //SI HAY SEMAFOROS EN SENTIDO CONTRARIO SE DUPLICAN
				for _, sem := range cDerecha.Objects {
					if sem, ok := sem.(*canvas.Image); ok {
						cIzquierda.Add(sem)
					}
				}
			}
		} else {
			layoutComponentes.Refresh()
		}

		//BORRAR SEMAFORO DEL OTRO LADO AL AÑADIR CARRILES EN ESTE SENTIDO
		if numCarrilesPrevios[1] == 0 && cIzquierda != nil && len(cIzquierda.Objects) > 0 {
			cIzquierda.Objects = nil
			cIzquierda.Refresh()
		}

		numCarrilesPrevios[1] = numCarrilesFuera
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
		cDerecha, cIzquierda = menuAddSemaforos(dir, w, pasoPeatonesActivado, numCarrilesCentro, numCarrilesFuera, cDerecha, cIzquierda)
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
				numCarrilesPrevios[0] = numCarrilesCentro
				numCarrilesPrevios[1] = numCarrilesFuera
			}
		}
		sliderCarrilesHaciaFuera.Max = float64(4 - numCarrilesCentro)
		sliderCarrilesHaciaFuera.Refresh()

		sliderCarrilesHaciaCentro.Max = float64(4 - numCarrilesFuera)
		sliderCarrilesHaciaCentro.Refresh()
	}

	//SI YA HAY SEMÁFOROS SE ACTUALIZAN LOS CONTENEDORES
	for _, l := range layoutComponentes.Objects {
		switch dir {
		case 1, 2, 3, 4:
			if l.Position() == posSemIzqda[dir-1] {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
				}
			} else if l.Position() == posSemDcha[dir-1] {
				if l, ok := l.(*fyne.Container); ok {
					cDerecha = l
				}
			}
		}
	}

	d := dialog.NewCustom(fmt.Sprintf("Editar dirección %d", dir), "Cerrar", c, w)
	d.Show()
}

// Colocación de pasos de peatones
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

// Colocación de carriles
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

// Menú para cambiar el tipo de flecha de los carriles entrantes
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

// Menú para añadir y seleccionar semáforos
func menuAddSemaforos(dir int, w fyne.Window, peatones bool, numCarrilesCentro, numCarrilesFuera int, cDerecha, cIzquierda *fyne.Container) (*fyne.Container, *fyne.Container) {
	cEspacio := container.New(layout.NewGridLayout(4))
	cBotones := container.New(layout.NewGridLayout(4))
	existen := false

	//COMPROBAR EXISTENTES
	for _, l := range layoutComponentes.Objects {
		switch dir {
		case 1, 2, 3, 4:
			if l.Position() == posSemIzqda[dir-1] {
				if l, ok := l.(*fyne.Container); ok {
					cIzquierda = l
					existen = true
				}
			} else if l.Position() == posSemDcha[dir-1] {
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

		cIzquierda.Move(posSemIzqda[dir-1])
		cDerecha.Move(posSemDcha[dir-1])

		cDerecha.Resize(sizeLayout)
		cIzquierda.Resize(sizeLayout)
	}

	//BOTON DE CREACION DE SEMAFOROS
	var btnAddSemaforoFuera, btnAddSemaforoDentro /*, btnAddSemaforoPeatones*/ *widget.Button

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
				menuSecuenciaSemaforos(dir, w, false, sem, getPosicionEnArray(sem, cIzquierda.Objects), func() {
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
			var b *widget.Button
			b = widget.NewButton("Semáforo saliente", func() {
				if sem, ok := cIzquierda.Objects[0].(*canvas.Image); ok {
					menuSecuenciaSemaforos(dir, w, false, sem, getPosicionEnArray(sem, cIzquierda.Objects), func() {
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
			var b *widget.Button
			b = widget.NewButton("Semáforo entrante", func() {
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

				var b *widget.Button
				b = widget.NewButton("Semáforo entrante", func() {
					if sem, ok := obj.(*canvas.Image); ok {
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

	//FUNCIONALIDAD FUTURA: SEMÁFOROS DE PEATONES
	/*
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
	*/

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

	return cDerecha, cIzquierda
}

// Estructura que guarda los datos de un semáforo
type Secuencia struct {
	Semaforo  *canvas.Image `json:"sem"`
	Direccion int           `json:"dir"`
	Colores   []string      `json:"colores"`
	Segundos  []int         `json:"segundos"`
	Posicion  int           `json:"pos"`
	Saliente  bool          `json:"saliente"`
	DirFlecha int           `json:"dirflecha"`
}

// Mostrar el menú de secuencias de un semáforo
func menuSecuenciaSemaforos(dir int, w fyne.Window, entrante bool, sem *canvas.Image, pos int, onDelete func()) { //onDelete borra el botón al borrar el semáforo
	c := container.NewVBox()
	var opciones []string
	cSecuencia := container.New(layout.NewGridLayout(3))
	dirFlecha := 0

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
				dirFlecha = 0
			case "Derecha":
				sem.Image = semaforos[6]
				sem.Refresh()
				dirFlecha = 1
			case "Frente":
				sem.Image = semaforos[9]
				sem.Refresh()
				dirFlecha = 2
			case "Izquierda":
				sem.Image = semaforos[12]
				sem.Refresh()
				dirFlecha = 3
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
			if sec.Posicion == pos && sec.Saliente == !entrante {
				coloresUsados = colocarFases(coloresUsados, sec, cSecuencia)
				break
			}
		}
	}

	cBotones := container.New(layout.NewGridLayout(5))

	btnCopiar := widget.NewButton("Copiar fases", func() {
		fasesCopiada = obtenerSecuencia(cSecuencia, sem, dir, dirFlecha, pos, w, false)
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
		s := obtenerSecuencia(cSecuencia, sem, dir, dirFlecha, pos, w, !entrante)

		//SE ALMACENAN LAS SECUENCIAS, SI YA ESTABAN ALMACENADAS SE SOBREESCRIBEN
		existe := false
		for i, sec := range secuencias[dir] {
			if sec.Posicion == s.Posicion && sec.Saliente == s.Saliente && sec.Semaforo == s.Semaforo {
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

// Crear la estructura de secuencia para un semáforo
func obtenerSecuencia(cSecuencia *fyne.Container, sem *canvas.Image, dir, dirFlecha, pos int, w fyne.Window, saliente bool) Secuencia {
	var colores []string
	var segundos []int

	for _, fase := range cSecuencia.Objects {
		if comboBox, ok := fase.(*widget.Select); ok {
			colores = append(colores, comboBox.Selected)
		} else if entry, ok := fase.(*widget.Entry); ok {
			if entry.Text == "" {
				entry.Text = "0"
			}
			if i, err := strconv.Atoi(entry.Text); err == nil {
				segundos = append(segundos, i)
			} else {
				mostrarError(err.Error(), fyne.CurrentApp(), w)
			}
		}
	}

	s := Secuencia{
		sem,
		dir,
		colores,
		segundos,
		pos,
		saliente,
		dirFlecha,
	}

	return s
}

// Colocar en el menú de secuencias las fases ya creadas
func colocarFases(coloresUsados []string, sec Secuencia, cSecuencia *fyne.Container) []string {
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
		inputTiempo.Text = strconv.Itoa(int(sec.Segundos[i]))
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

// Colocar los colores posibles en los ComboBox de las secuencias
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

// Activar la ejecución de las secuencias de semáforos
func ejecucion(cont *fyne.Container) {
	startTickerBroadcast()
	tickerEjecucion.Stop()
	for _, secuenciasDir := range secuencias {
		for _, secuencia := range secuenciasDir {
			//comprobar que la secuencia esté rellena
			if secuencia.Direccion > 0 {
				go func(cont *fyne.Container) {
					for {
						for i, c := range secuencia.Colores {
							select {
							case <-chanParar: //parar
								return
							default:
								switch c {

								case "Verde":
									secuencia.Semaforo.Image = semaforos[(3*secuencia.DirFlecha)+1]
									fyne.Do(func() { secuencia.Semaforo.Refresh() })
								case "Ámbar":
									secuencia.Semaforo.Image = semaforos[(3*secuencia.DirFlecha)+2]
									fyne.Do(func() { secuencia.Semaforo.Refresh() })
								case "Ámbar (parpadeo)":
									ambar(secuencia.Semaforo, secuencia.DirFlecha, secuencia.Segundos[i])
								case "Rojo":
									secuencia.Semaforo.Image = semaforos[(3*secuencia.DirFlecha)+3]
									fyne.Do(func() { secuencia.Semaforo.Refresh() })
								}

								if c != "Ámbar (parpadeo)" { //el ámbar parpadeante cambia en ambar()
									for s := 0; s < secuencia.Segundos[i]; s++ {
										select {
										case <-chanParar:
											return
										case <-tickBroadcast:
										}
									}
								}
							}
						}
					}
				}(cont)
			}
		}
	}
	tickerEjecucion.Reset(time.Second)
}

// Comprobación de si se ha pausado para mandar una señal a todos los hilos
func startTickerBroadcast() {
	go func() {
		for {
			select {
			case <-tickerEjecucion.C:
				if pausado {
					//si se pausa espera a que se quite para seguir
					for pausado {
						time.Sleep(50 * time.Millisecond)
					}
				}
				close(tickBroadcast)
				tickBroadcast = make(chan struct{})
			}
		}
	}()
}

// Parar la ejecución
func pararEjecucion() {
	pausado = false
	tickerEjecucion.Stop()
	close(chanParar)
	chanParar = make(chan struct{})
	tickBroadcast = make(chan struct{})
	tickerEjecucion = time.NewTicker(time.Second)

	//VOLVER A COLOCAR LOS SEMÁFOROS EN ROJO CON SU FLECHA (SI TIENE)
	for _, secuenciasDir := range secuencias {
		for _, secuencia := range secuenciasDir {
			//comprobar que la secuencia esté rellena
			if secuencia.Direccion > 0 {
				secuencia.Semaforo.Image = semaforos[(3*secuencia.DirFlecha)+3]
				secuencia.Semaforo.Refresh()
			}
		}

	}
}

// Guardar diseño en BBDD
func guardarBBDD(nombreDesign string) {
	db, err := sql.Open("sqlite3", home+"/.intersecciones/bbdd.db")

	if err != nil {
		mostrarError(err.Error(), fyne.CurrentApp())
	}

	defer db.Close()

	if nombre != "" { //DISEÑO YA CREADO, SE BORRA EL ANTERIOR
		_, err = db.Exec("DELETE FROM Intersecciones WHERE id = ?", id)
		if err != nil {
			mostrarError("Error al borrar el diseño anterior: "+err.Error(), fyne.CurrentApp())
			return
		}
	}

	//INSERTAR EN BBDD
	_, err = db.Exec("INSERT INTO Intersecciones (nombre, num_direcciones) VALUES (?, ?)", nombreDesign, numDirecciones)
	if err != nil {
		mostrarError("Error al borrar el diseño anterior: "+err.Error(), fyne.CurrentApp())
		return
	}

	//OBTENER EL ID DEL DISEÑO INSERTADO
	rows, err := db.Query("SELECT id FROM Intersecciones WHERE nombre = ?", nombreDesign)
	if err != nil {
		mostrarError("Error al obtener el ID del diseño: "+err.Error(), fyne.CurrentApp())
		return
	}

	var id int
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			mostrarError("Error al leer el ID del diseño: "+err.Error(), fyne.CurrentApp())
			return
		}
	}

	//INSERTAR DIRECCIONES
	for dir := 1; dir <= numDirecciones; dir++ {
		_, err = db.Exec("INSERT INTO Direcciones (interseccion_id, direccion, tiene_paso_peatones) VALUES (?, ?, ?)", id, dir, pasosPeatones[dir-1])
		if err != nil {
			mostrarError("Error al insertar la dirección: "+err.Error(), fyne.CurrentApp())
			return
		}

		//OBTENER ID DE LA DIRECCIÓN INSERTADA
		rows, err = db.Query("SELECT id FROM Direcciones WHERE interseccion_id = ? AND direccion = ?", id, dir)
		if err != nil {
			mostrarError("Error al obtener el ID de la dirección: "+err.Error(), fyne.CurrentApp())
			return
		}

		var idDir int
		for rows.Next() {
			err = rows.Scan(&idDir)
			if err != nil {
				mostrarError("Error al leer el ID de la dirección: "+err.Error(), fyne.CurrentApp())
				return
			}
		}

		//INSERTAR SEMÁFOROS
		if secuencias[dir-1] != nil {
			for _, secuencia := range secuencias[dir-1] {
				if secuencia.Semaforo == nil {
					continue //si no hay semáforo no se guarda nada
				}

				colores := ""
				if len(secuencia.Colores) > 0 {
					colores = strings.Join(secuencia.Colores, ",")
				}

				segundos := ""
				if len(secuencia.Segundos) > 0 {
					for i, seg := range secuencia.Segundos {
						if i == 0 {
							segundos += strconv.Itoa(seg)
						} else {
							segundos += "," + strconv.Itoa(seg)
						}
					}
				}

				_, err = db.Exec(`INSERT INTO Semaforos (interseccion_id, direccion_id, colores, segundos, 
				posicion, saliente, dir_flecha) VALUES (?, ?, ?, ?, ?, ?, ?)`,
					id, idDir, colores, segundos, secuencia.Posicion, secuencia.Saliente, secuencia.DirFlecha)
				if err != nil {
					mostrarError("Error al insertar el semáforo: "+err.Error(), fyne.CurrentApp())
					return
				}
			}
		}

		//INSERTAR CARRILES
		if layoutsMarcas[dir-1] != nil {
			for _, obj := range layoutsMarcas[dir-1].Objects {
				if img, ok := obj.(*canvas.Image); ok {
					dirFlecha := getPosicionFlecha(img.Image, flechas)

					if dirFlecha != -1 {
						sentidoFlecha := 0
						if dirFlecha >= 8 && dirFlecha < 11 { //HACIA FUERA
							sentidoFlecha = 1
						}

						_, err = db.Exec("INSERT INTO Carriles (interseccion_id, direccion_id, sentido, tipo_flecha) VALUES (?, ?, ?, ?)",
							id, idDir, sentidoFlecha, dirFlecha%8)
						if err != nil {
							mostrarError("Error al insertar el carril: "+err.Error(), fyne.CurrentApp())
							return
						}
					}
				}
			}
		}
	}
}

// Mostrar diseños guardados
func abrirBBDD(c *fyne.Container, w fyne.Window, d *dialog.CustomDialog, image *canvas.Image) {
	db, err := sql.Open("sqlite3", home+"/.intersecciones/bbdd.db")
	if err != nil {
		mostrarError("Error al abrir la base de datos: "+err.Error(), fyne.CurrentApp())
		return
	}
	defer db.Close()

	//COMPROBAR SI HAY DISEÑOS
	query := "SELECT COUNT(*) FROM Intersecciones"
	rows, err := db.Query(query)
	if err != nil {
		mostrarError("Error al consultar la base de datos: "+err.Error(), fyne.CurrentApp())
		return
	}
	c.Objects = c.Objects[:0] //limpiar container

	for rows.Next() {
		var num int

		err = rows.Scan(&num)

		if err != nil {
			mostrarError("Error al leer la base de datos: "+err.Error(), fyne.CurrentApp())
			return
		}

		if num == 0 {
			mostrarError("No hay diseños guardados", fyne.CurrentApp())
			return
		}
	}

	query = "SELECT id, nombre FROM Intersecciones"
	rows, err = db.Query(query)
	if err != nil {
		mostrarError("Error al consultar la base de datos: "+err.Error(), fyne.CurrentApp())
		return
	}
	c.Objects = c.Objects[:0] //limpiar container

	for rows.Next() {
		var nombreDiseno string
		var idDiseno int

		err = rows.Scan(&idDiseno, &nombreDiseno)

		if err != nil {
			mostrarError("Error al leer la base de datos: "+err.Error(), fyne.CurrentApp())
			return
		}

		var b1, b2 *widget.Button

		b1 = widget.NewButton(nombreDiseno, func() {
			d.Hide()
			id = idDiseno //guardar el id del diseño seleccionado
			nombre = nombreDiseno
			colocarTodo(image, w)
		})

		b2 = widget.NewButton("Borrar", func() { //borrar este diseño
			dialog.ShowConfirm("Borrar diseño", "¿Seguro que quieres borrar este diseño?", func(b bool) {
				if b {
					db, err := sql.Open("sqlite3", home+"/.intersecciones/bbdd.db")
					if err != nil {
						mostrarError("Error al abrir la base de datos: "+err.Error(), fyne.CurrentApp())
						return
					}
					defer db.Close()

					_, err = db.Exec("PRAGMA foreign_keys = ON")
					if err != nil {
						mostrarError("Error con la base de datos: "+err.Error(), fyne.CurrentApp())
						return
					}

					_, err = db.Exec("DELETE FROM Intersecciones WHERE id = ?", idDiseno)
					if err != nil {
						mostrarError("Error al borrar el diseño anterior: "+err.Error(), fyne.CurrentApp())
						return
					}

					for i, obj := range c.Objects {
						if hbox, ok := obj.(*fyne.Container); ok {
							if len(hbox.Objects) == 2 && hbox.Objects[0] == b1 && hbox.Objects[1] == b2 {
								c.Objects = append(c.Objects[:i], c.Objects[i+1:]...)
								break
							}
						}
					}
					c.Refresh()
				}
			}, w)
		})

		c.Objects = append(c.Objects, container.NewHBox(b1, b2))
	}
}

// Colocar componentes al abrir un diseño de BBDD
func colocarTodo(image *canvas.Image, w fyne.Window) {
	db, err := sql.Open("sqlite3", home+"/.intersecciones/bbdd.db")
	if err != nil {
		mostrarError("Error al abrir la base de datos: "+err.Error(), fyne.CurrentApp())
		return
	}
	defer db.Close()

	//guardar número de direcciones
	row := db.QueryRow("SELECT num_direcciones FROM Intersecciones WHERE id = ?", id)
	err = row.Scan(&numDirecciones)
	if err != nil {
		mostrarError("Error al leer el número de direcciones: "+err.Error(), fyne.CurrentApp())
		return
	}

	if numDirecciones == 3 {
		image.Image = fondos[0]
		image.Refresh()
		colocarBotones(w, layoutBotonesEditar, numDirecciones, false, modoEdicion)
	} else {
		colocarBotones(w, layoutBotonesEditar, numDirecciones, false, modoEdicion)
	}

	//cargar direcciones y pasos de peatones
	type Direccion struct {
		ID           int
		Num          int
		PasoPeatones bool
	}
	dirs := make([]Direccion, numDirecciones)
	rows, err := db.Query("SELECT id, direccion, tiene_paso_peatones FROM Direcciones WHERE interseccion_id = ?", id)
	if err != nil {
		mostrarError("Error al leer las direcciones: "+err.Error(), fyne.CurrentApp())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var d Direccion
		var pasoInt int
		err = rows.Scan(&d.ID, &d.Num, &pasoInt)
		if err != nil {
			mostrarError("Error al leer dirección: "+err.Error(), fyne.CurrentApp())
			return
		}
		d.PasoPeatones = pasoInt != 0
		if d.Num >= 1 && d.Num <= 4 {
			dirs[d.Num-1] = d
			pasosPeatones[d.Num-1] = d.PasoPeatones
		}
	}

	//cargar carriles y flechas
	for i, d := range dirs {
		if d.ID == 0 {
			continue
		}
		rowsCarril, err := db.Query("SELECT sentido, tipo_flecha FROM Carriles WHERE direccion_id = ?", d.ID)
		if err != nil {
			mostrarError("Error al leer los carriles: "+err.Error(), fyne.CurrentApp())
			return
		}
		defer rowsCarril.Close()
		numCentro, numFuera := 0, 0
		var tiposCentro, tiposFuera []int
		for rowsCarril.Next() {
			var sentido, tipo int
			err = rowsCarril.Scan(&sentido, &tipo)
			if err != nil {
				mostrarError("Error al leer carril: "+err.Error(), fyne.CurrentApp())
				return
			}
			if sentido == 0 {
				numCentro++
				tiposCentro = append(tiposCentro, tipo)
			} else {
				numFuera++
				tiposFuera = append(tiposFuera, tipo)
			}
		}

		modificarNumCarriles(i+1, numCentro, numFuera)

		idxCentro, idxFuera := 0, 0
		if layoutsMarcas[i] != nil {
			for _, obj := range layoutsMarcas[i].Objects {
				img, ok := obj.(*canvas.Image)
				if !ok {
					continue
				}

				posFlecha := getPosicionFlecha(img.Image, flechas)
				if posFlecha == -1 {
					continue
				}
				if idxFuera < len(tiposFuera) {
					if tiposFuera[idxFuera] >= 0 && tiposFuera[idxFuera] < 8 {
						img.Image = flechas[tiposFuera[idxFuera]+8*i]
						img.Refresh()
					}
					idxFuera++
				} else if idxCentro < len(tiposCentro) {
					if tiposCentro[idxCentro] >= 0 && tiposCentro[idxCentro] < 8 {
						img.Image = flechas[tiposCentro[idxCentro]+8*i]
						img.Refresh()
					}
					idxCentro++
				}
			}
		}
	}

	//cargar semáforos y secuencias
	rows, err = db.Query("SELECT direccion_id, colores, segundos, posicion, saliente, dir_flecha FROM Semaforos WHERE interseccion_id = ?", id)
	if err != nil {
		mostrarError("Error al leer los semáforos: "+err.Error(), fyne.CurrentApp())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var dirID, posicion, saliente, dirFlecha int
		var coloresStr, segundosStr string
		err = rows.Scan(&dirID, &coloresStr, &segundosStr, &posicion, &saliente, &dirFlecha)
		if err != nil {
			mostrarError("Error al leer semáforo: "+err.Error(), fyne.CurrentApp())
			return
		}

		// Busca la dirección real (1-4) para este dirID
		dirNum := -1
		for _, d := range dirs {
			if d.ID == dirID {
				dirNum = d.Num - 1 // d.Num es la dirección real
				break
			}
		}
		if dirNum == -1 {
			continue
		}

		img := canvas.NewImageFromImage(semaforos[(3*dirFlecha)+3])
		img.FillMode = canvas.ImageFillOriginal
		img.Resize(semaforoSize)

		var numCentro, numFuera int
		rowCarril, err := db.Query("SELECT sentido FROM Carriles WHERE direccion_id = ?", dirID)
		if err == nil {
			for rowCarril.Next() {
				var sentido int
				rowCarril.Scan(&sentido)
				if sentido == 0 {
					numCentro++
				} else if sentido == 1 {
					numFuera++
				}
			}
			rowCarril.Close()
		}

		if saliente == 1 {
			if layoutsSemaforos[dirNum] == nil {
				layoutsSemaforos[dirNum] = container.New(&layouts.Semaforos{})
				layoutsSemaforos[dirNum].Move(posSemIzqda[dirNum-1])
				layoutsSemaforos[dirNum].Resize(fyne.NewSize(90, 100))
			}
			layoutsSemaforos[dirNum].Add(img)
			if numCentro == 0 {
				if layoutsSemaforos[dirNum+4] == nil {
					layoutsSemaforos[dirNum+4] = container.New(&layouts.Semaforos{})
					layoutsSemaforos[dirNum+4].Move(posSemDcha[dirNum-1])
					layoutsSemaforos[dirNum+4].Resize(fyne.NewSize(90, 100))
				}
				layoutsSemaforos[dirNum+4].Add(img)
			}
		} else {
			if layoutsSemaforos[dirNum+4] == nil {
				layoutsSemaforos[dirNum+4] = container.New(&layouts.Semaforos{})
				layoutsSemaforos[dirNum+4].Move(posSemDcha[dirNum-1])
				layoutsSemaforos[dirNum+4].Resize(fyne.NewSize(90, 100))
			}
			layoutsSemaforos[dirNum+4].Add(img)
			if numFuera == 0 {
				if layoutsSemaforos[dirNum] == nil {
					layoutsSemaforos[dirNum] = container.New(&layouts.Semaforos{})
					layoutsSemaforos[dirNum].Move(posSemIzqda[dirNum-1])
					layoutsSemaforos[dirNum].Resize(fyne.NewSize(90, 100))
				}
				layoutsSemaforos[dirNum].Add(img)
			}
		}

		coloresArr := strings.Split(coloresStr, ",")
		segundosArr := strings.Split(segundosStr, ",")
		var segundosInt []int
		for _, s := range segundosArr {
			val, _ := strconv.Atoi(s)
			segundosInt = append(segundosInt, val)
		}
		sec := Secuencia{
			Semaforo:  img,
			Direccion: dirNum,
			Colores:   coloresArr,
			Segundos:  segundosInt,
			Posicion:  posicion,
			Saliente:  saliente == 1,
			DirFlecha: dirFlecha,
		}
		if secuencias[dirNum] == nil {
			secuencias[dirNum] = []Secuencia{}
		}
		secuencias[dirNum] = append(secuencias[dirNum], sec)
	}

	//añadir layouts de semáforos y marcas al layout principal
	layoutComponentes.Objects = nil
	for i := 0; i < 4; i++ {
		if layoutsMarcas[i] != nil {
			layoutComponentes.Add(layoutsMarcas[i])
		}
		if layoutsSemaforos[i] != nil {
			layoutComponentes.Add(layoutsSemaforos[i])
		}
		if layoutsSemaforos[i+4] != nil {
			layoutComponentes.Add(layoutsSemaforos[i+4])
		}
	}

	//colocar pasos de peatones
	for i, d := range dirs {
		if d.PasoPeatones {
			modificarPasoPeatones(i+1, true)
		}
	}

	layoutComponentes.Refresh()
}

// Abrir menú de ayuda
func mostrarAyuda(w fyne.Window) {
	textoAyuda := widget.NewLabel(`Este programa consiste en 2 partes principales:
	- Edición: en la vista de edición se pueden activar los botones con los que
	poder modificar cada dirección, creando pasos de peatones, carriles, flechas
	y semáforos.
	- Ejecución: en la vista de ejecución se puede ver funcionando a los semáforos
	que se hayan colocado siguiendo las fases tal y como se hayan diseñado en bucle.
	
Guía de ejemplo para crear un diseño:
	Tras haber elegido el número de direcciones, podemos empezar a editar cada
	dirección. En los menús de edición podemos modificar todo lo mencionado 
	anteriormente. Lo ideal es elegir el número de carriles, hasta un máximo de 4, su 
	sentido y su dirección en primer lugar, junto con pasos de peatones si así lo 
	queremos. Una vez tenemos todos los carriles hechos, podemos empezar con los 
	semáforos.

	En el menú de añadir semáforos podemos crear los que queramos, teniendo en cuenta
	que sólo puede haber 1 en sentido saliente, y hasta 3 en sentido entrante. A su
	vez, se puede modificar la dirección de los semáforos entrantes. Si creamos más
	semáforos de los que queremos, entramos en el semáforo y lo podremos borrar.

	Además de la dirección, la función principal de la edición de los semáforos es
	poder cambiar sus secuencias. Las secuencias se dividen en fases: el tiempo que
	estará encendido cada color, colocados de forma ordenada, sabiendo que al terminar
	la última fase la secuencia comenzará de nuevo. Las fases de un semáforo se pueden
	copiar y pegar en otro, para así replicarlos fácilmente.

	Una vez tenemos todo creado y editado a nuestro gusto, podemos pasar a la vista de
	ejecución, donde se puede a ver los semáforos cambiando de color tal y como se lo
	hemos indicado en sus respectivas secuencias. La ejecución se puede parar y
	reiniciar según queramos. 
	
	Una vez hayamos terminado, podemos guardar el diseño completo pulsando el botón de
	arriba a la izquierda, eligiendo un nombre para el diseño. Si lo guardamos y luego
	seguimos editando, habrá que guardar de nuevo con el mismo nombre, para así
	sobreescribirlo.

	Para abrir un diseño guardado, hay que abrir el programa y elegirlo. Para borrarlo,
	hay que pulsar el botón de "Borrar" a la derecha del nombre del diseño en la pantalla
	de elección de diseño guardado.
	`)
	textoAyuda.Alignment = fyne.TextAlignLeading
	dialogAyuda := dialog.NewCustom("Ayuda", "Cerrar", textoAyuda, w)
	dialogAyuda.Show()
}

func salir(w fyne.Window) {
	dialogSalir := dialog.NewConfirm("Salir", "¿Seguro que quieres salir? Todo lo que no se haya guardado se perderá", func(b bool) {
		if b {
			w.Close()
		}
	}, w)
	dialogSalir.Show()
}

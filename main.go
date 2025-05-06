package main

import (
	"database/sql"
	"fmt"
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
	inicializado := false
	numDirecciones := 0
	botones := make([]*widget.Button, 0)

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
	semVerde := canvas.NewImageFromFile("images/semaforos/semVerde.png")
	semVerde.FillMode = canvas.ImageFillOriginal
	semVerde.Resize(semaforoSize)

	semAmbar := canvas.NewImageFromFile("images/semaforos/semAmbar.png")
	semAmbar.FillMode = canvas.ImageFillOriginal
	semAmbar.Resize(semaforoSize)

	semRojo := canvas.NewImageFromFile("images/semaforos/semRojo.png")
	semRojo.FillMode = canvas.ImageFillOriginal
	semRojo.Resize(semaforoSize)

	semApagado := canvas.NewImageFromFile("images/semaforos/semApagado.png")
	semApagado.FillMode = canvas.ImageFillOriginal
	semApagado.Resize(semaforoSize)

	//FONDO
	image := canvas.NewImageFromFile("images/cruces/cruce_vacio.png")
	image.FillMode = canvas.ImageFillOriginal

	layoutBotonesEditar := container.NewWithoutLayout()
	fondo := container.NewStack(image, layoutBotonesEditar)

	semTest := semAmbar
	layoutBotonesEditar.Add(semTest)
	semTest.Move(fyne.NewPos(500, 500))
	fyne.DoAndWait(func() { ambar(semAmbar, semApagado, semTest, layoutBotonesEditar) })

	//BARRAS DE HERRAMIENTAS
	barraHerramientasEdicion := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentSaveIcon(), func() {
			//HACER EN ACCESO A DATOS
			fmt.Println("guardar diseño")
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			if modoEdicion {
				modoEdicion = false
				colocarBotones(layoutBotonesEditar, botones, numDirecciones, inicializado, modoEdicion)
			} else {
				modoEdicion = true
				colocarBotones(layoutBotonesEditar, botones, numDirecciones, inicializado, modoEdicion)
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
			image = canvas.NewImageFromFile("images/cruces/cruce3_vacio.png")
			image.FillMode = canvas.ImageFillOriginal
			fondo.Objects[0] = image
			fondo.Refresh()
			colocarBotones(layoutBotonesEditar, botones, numDirecciones, inicializado, modoEdicion)
			dDir.Hide()
		})
		boton4 := widget.NewButton("4 direcciones", func() {
			numDirecciones = 4
			colocarBotones(layoutBotonesEditar, botones, numDirecciones, inicializado, modoEdicion)
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
	inicializado = true
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

func ambar(ambar, apagado, sem *canvas.Image, c *fyne.Container) {
	pos := getPosicion(sem, c)

	for i := 0; i < 10; i++ {
		c.Objects[pos] = ambar
		c.Objects[pos].Refresh()
		time.Sleep(time.Second)
		c.Objects[pos] = apagado
		c.Objects[pos].Refresh()
		time.Sleep(time.Second)
	}
}

func colocarBotones(c *fyne.Container, botones []*widget.Button, numDir int, inicializado, editar bool) {
	if !inicializado {
		for i := 0; i < numDir; i++ {
			boton := widget.NewButton("Editar "+fmt.Sprint(i+1), func() {
			})
			boton.Hidden = true
			botones = append(botones, boton)
			c.Add(boton)
			switch i {
			case 0:
				boton.Move(fyne.NewPos(100, 300))
			case 1:
				boton.Move(fyne.NewPos(300, 100))
			case 2:
				boton.Move(fyne.NewPos(100, 100))
			case 3:
				boton.Move(fyne.NewPos(300, 300))
			}
		}
	} else {
		fmt.Println("mostrando:", editar)

		for _, boton := range botones {
			boton.Hidden = editar
			boton.Refresh()
		}
	}
}

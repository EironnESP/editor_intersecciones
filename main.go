package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/kbinani/screenshot"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	a := app.New()

	//VARIABLES GLOBALES
	home, _ := os.UserHomeDir()

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

	//CARGAR FONDO
	image := canvas.NewImageFromFile("images/cruces/cruce_vacio.png")
	image.FillMode = canvas.ImageFillOriginal
	w.SetContent(image)
	w.Show()

	//PANTALLA NUEVO DISEÑO O EXISTENTE
	w1 := a.NewWindow("Abrir diseño")
	w1.Resize(fyne.NewSize(200, 80))
	w1.CenterOnScreen()
	w1.SetIcon(icono)

	var c *fyne.Container

	botonNuevo := widget.NewButton("Nuevo diseño", func() {
		defer w1.Close()
		fmt.Println("nuevo")
	})

	//HACER EN ACCESO A DATOS
	botonAbrir := widget.NewButton("Abrir diseño", func() {
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
				mostrarError("No hay diseños guardados", a)
				w1.Close()
				return
			}

			if err != nil {
				mostrarError("Error al leer la base de datos: "+err.Error(), a)
				return
			}

			fmt.Println(nombre)
			c.Objects = append(c.Objects, widget.NewButton(nombre, func() {
				defer w1.Close()
				fmt.Println("abrir diseño: " + nombre)

			}))
		}
	})

	c = container.New(layout.NewVBoxLayout(), botonNuevo, botonAbrir)
	w1.SetContent(c)
	w1.Show()

	a.Run()
}

func mostrarError(e string, a fyne.App) {
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

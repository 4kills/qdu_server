package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
)

//---------------------------------------------------------
// Web-Server-Code startet hier
//---------------------------------------------------------

var tmpl *template.Template

// Hauptfunktion des Webservers
func webServer() {
	// Wartet bis nötige variablen vom Benutzer gesetzt sind
	log.Print("Web-Server launched...\n\n")

	// assign assets to handler
	http.HandleFunc(config.DirectoryWeb, handleRequest)
	http.Handle("/pics/", http.StripPrefix("/pics/", http.FileServer(http.Dir("./pics"))))
	tmpl = template.Must(template.ParseFiles("gallery.html"))
	log.Print("Successfully assigned assets to web-server")

	go http.ListenAndServe(":http", nil)
	log.Fatal("Web-Server crashed: \n\n", http.ListenAndServeTLS(config.PortWeb,
		"/etc/letsencrypt/live/haveachin.de/fullchain.pem",
		"/etc/letsencrypt/live/haveachin.de/privkey.pem", nil))
}

// Die Funktion die aufgerufen wird, wenn eine http-Anfrage hereinkommt
func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Liest aus der URL durch die GET-Methode das angefragte Bild aus
	keys := r.URL.Query()
	pic, okI := keys["i"]
	tokstr, okMe := keys["me"]
	if ((okI || okMe) == false) || (okI && okMe) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if okMe {
		sendGallery(w, tokstr[0])
		return
	}

	showPic(w, pic[0])
}

func showPic(w http.ResponseWriter, picName string) {
	// schreibt kompletten inhalt der Bild-Datei in den RAM
	dat, err := ioutil.ReadFile(filepath.Join(config.DirectoryPics, picName+".png"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		log.Println(err)
		return
	}

	// Sendet das Bild als Byte-Stream zum Broswer des Benutzers
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(dat)))
	if _, err := w.Write(dat); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	pic, err := uuid.Parse(picName)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		log.Println("picName decode error:", err)
		return
	}

	_, err = db.Exec("UPDATE pics SET clicks = clicks + 1 WHERE pic_id = ?", pic[:])
	if err != nil {
		log.Println("db update(increment clicks) error:", err)
	}
}

type user struct {
	Pics []pic
}
type pic struct {
	Name   string
	Time   string
	Clicks int
}

func sendGallery(w http.ResponseWriter, tokstr string) {
	tok, err := uuid.Parse(tokstr)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		log.Println("tokstr decode error:", err)
		return
	}

	rows, err := db.Query(
		`SELECT pic_id, timestamp, clicks 
		FROM pics 
		WHERE token = ? 
		ORDER BY timestamp DESC`, tok[:])
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Println("db select query error:", err)
		return
	}

	var u user

	for rows.Next() {
		var (
			nam16 []byte
			ts    rawTime
			p     pic
		)

		if err := rows.Scan(&nam16, &ts, &p.Clicks); err != nil {
			log.Println("scan row error:", err)
			continue
		}

		picID, err := uuid.FromBytes(nam16)
		if err != nil {
			log.Println("nam16 encode error:", err)
			continue
		}

		temp, _ := ts.unify()
		p.Name = picID.String()
		p.Time = temp.UTC().Format("02-01-2006 15:04:05")

		u.Pics = append(u.Pics, p)
	}

	tmpl.Execute(w, u)
}

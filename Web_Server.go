package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
)

//---------------------------------------------------------
// Web-Service
//---------------------------------------------------------

var tmpl *template.Template

func webServer() {
	log.Print("Web-Server launched...\n\n")

	// assign assets to handler
	http.HandleFunc(config.DirectoryWeb, handleRequest)
	http.Handle("/pics/", http.StripPrefix("/pics/", http.FileServer(http.Dir("./pics"))))
	tmpl = template.Must(template.ParseFiles("gallery.html"))
	log.Print("Successfully assigned assets to web-server")

	if config.Fullchain != "" {
		go http.ListenAndServe(":http", nil)
		log.Fatal("Web-Server crashed: \n\n", http.ListenAndServeTLS(config.PortWeb,
			config.Fullchain,
			config.Privkey, nil))
	} else {
		log.Fatal("Web-Server crashed: \n\n", http.ListenAndServe(":http", nil))
	}
}

// called upon http requests
func handleRequest(w http.ResponseWriter, r *http.Request) {
	// GET the requested picture
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
	// writes picture into ram
	dat, err := ioutil.ReadFile(filepath.Join(config.DirectoryPics, picName+".png"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		log.Println(err)
		return
	}

	// sends pic as byte stream to browser
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(dat)))
	if _, err := w.Write(dat); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	pic, ok := legacyStrToUUID(w, picName)
	if !ok {
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
	tok, ok := legacyStrToUUID(w, tokstr)
	if !ok {
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
		t := temp.UTC()
		p.Time = t.Format("02-01-2006 15:04:05")
		legacyDate, _ := time.Parse("02-01-2006 15:04:05", "15-09-2018 12:00:00")
		if t.After(legacyDate) {
			p.Name = enc.Encode(picID[:])
		} else {
			p.Name = picID.String()
		}

		u.Pics = append(u.Pics, p)
	}

	tmpl.Execute(w, u)
}

func legacyStrToUUID(w http.ResponseWriter, tokstr string) (uuid.UUID, bool) {
	var tok uuid.UUID
	var err error

	switch len(tokstr) {
	case 36:
		tok, err = uuid.Parse(tokstr)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			log.Println("tokstrlen36 decode error:", err)
			return tok, false
		}
	case 22:
		b, err := enc.Decode(tokstr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("tokstrlen22 decode error:", err)
			return tok, false
		}

		tok, err = uuid.FromBytes(b[:])
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			log.Println("tokstrlen22 decode error:", err)
			return tok, false
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("tokstringlen unavailable, request rejected")
		return tok, false
	}

	return tok, true
}

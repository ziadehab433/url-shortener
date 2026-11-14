package main

import (
	"database/sql"
	json "encoding/json/v2"
	"log/slog"
	"net/http"
	"os"

	b62 "github.com/jxskiss/base62"

	_ "modernc.org/sqlite"
)

const address = "localhost:8080"

type reqBody struct {
	Url string `json:"url"`
}

type server struct {
	db *sql.DB
}

func (s *server) HandleShorten(w http.ResponseWriter, r *http.Request) {
	var reqBody reqBody
	err := json.UnmarshalRead(r.Body, &reqBody)
	if err != nil {
		w.Write([]byte("couldn't parse request body, it should look like this { 'url': 'https://example.com' }" + err.Error()))
		return
	}

	url := reqBody.Url
	res, err := s.db.Exec(`
		INSERT INTO urls (url) values (?)
	`, url)
	if err != nil {
		slog.Warn("coudln't insert url: " + url + " into db " + err.Error())
	}

	id, err := res.LastInsertId()
	if err != nil {
		slog.Warn("couldn't fetch last inserted's id " + err.Error())
	}

	newUrl := address + "/" + string(b62.FormatInt(id))
	w.Write([]byte(newUrl))
}

func (s *server) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	base62 := r.PathValue("string")
	id, err := b62.ParseInt([]byte(base62))
	if err != nil {
		w.Write([]byte("invalid base62 id"))
		return
	}

	row := s.db.QueryRow("SELECT url FROM urls WHERE id = ?", id)
	var url string
	if err := row.Scan(&url); err != nil {
		w.Write([]byte("there are no target urls for " + address + "/" + base62 + " (target urls get deleted after 2 days)"))
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusMovedPermanently)
}

func (s *server) HandleHomePage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
	Welcome to Url-shortener to shorten a url send a POST request to /shorten
	with the url in the payload like this example { 'url': 'https://example.com' }
	`))
}

func main() {
	db, err := sql.Open("sqlite", "bridge.db")
	if err != nil {
		slog.Error("couldn't open db " + err.Error())
		os.Exit(1)
	}
	initDB(db)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	server := &server{db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", server.HandleShorten)
	mux.HandleFunc("GET /{string}", server.HandleRedirect)
	mux.HandleFunc("GET /", server.HandleHomePage)

	slog.Info("server listening on " + address + "...")
	err = http.ListenAndServe(address, mux)
	if err != nil {
		slog.Error("error listening on socket " + address + " " + err.Error())
	}
}

func initDB(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS urls ( 
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			created_at INTEGER DEFAULT (unixepoch())
		)
	`)
	if err != nil {
		slog.Error("couldn't initialise db " + err.Error())
		os.Exit(1)
	}
}

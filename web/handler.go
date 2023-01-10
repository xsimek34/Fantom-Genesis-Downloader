package web

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
)

//
//go:embed content
var content embed.FS

// WebContentHandler handles the static content of the website
// using resources stored within the embedded FS.
func WebContentHandler() http.Handler {
	// drop the prefix of the root dir; make a simple file server to handle the requests
	root, err := fs.Sub(content, "content")
	if err != nil {
		fmt.Errorf("embedded root folder not found; %s", err.Error())
		return nil
	}
	fileServer := http.FileServer(http.FS(root))

	// make the handler
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// index page requested?
		if req.URL.Path == "/" {
			serveIndexPage(rw, req)
			return
		}

		// serve all the rest of the static content
		fileServer.ServeHTTP(rw, req)
	})
}

// serveIndexPage serves embedded index.html page response.
func serveIndexPage(rw http.ResponseWriter, req *http.Request) {
	f, err := content.Open("content/index.html")
	if err != nil {
		fmt.Errorf("embedded index.html file not found; %s", err.Error())
		http.Error(rw, "expected content missing", http.StatusInternalServerError)
		return
	}

	defer func(f fs.File) {
		if err := f.Close(); err != nil {
			fmt.Errorf("can not close index.html file; %s", err.Error())
		}
	}(f)

	stat, err := f.Stat()
	if err != nil {
		fmt.Errorf("can not stat embedded index.html; %s", err.Error())
		http.Error(rw, "content status not available", http.StatusInternalServerError)
		return
	}

	http.ServeContent(rw, req, "index.html", stat.ModTime(), f.(io.ReadSeeker))
}

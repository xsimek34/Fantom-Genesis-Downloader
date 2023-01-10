package downloader

import (
	"Fantom-Genesis-Generator/api"
	"Fantom-Genesis-Generator/web"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type unit struct {
	name string
	size int
}

type NewUnit struct {
	Index    int    `json:"index"`
	Epoch    int    `json:"epoch"`
	Category string `json:"category"`
	Password string `json:"pw"`
}

type CheckHash struct {
	Hash     string `json:"hash"`
	Password string `json:"password"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

type CheckServer struct {
	Category string
	Type     string
	Server   string
	Hash     string
}

type CheckServers struct {
	CheckServers []CheckServer
	sync.RWMutex
}

var checkServers CheckServers
var configPath string
var bufferSize int
var db api.Db
var config api.Config

func Init(path string) {
	configPath = path
	config = loadConfig()

	db.Lock()
	defer db.Unlock()

	db = loadDb()

	for _, c := range config.Categories {
		for i := range c.StaticFiles {
			c.StaticFiles[i].Static = true
		}
	}

	api.UpdateData(&db, &config)

	serve()
}

func loadConfig() api.Config {
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("can not open file")
	}

	data := api.Config{}
	json.Unmarshal(file, &data)
	bufferSize = data.BufferSize

	checkServers.Lock()
	defer checkServers.Unlock()

	for _, c := range config.Categories {
		for _, cs := range c.CompareServers {
			for _, f := range c.DynamicFiles {
				checkServers.CheckServers = append(checkServers.CheckServers, CheckServer{
					Category: c.Name,
					Type:     f.Type,
					Server:   cs,
					Hash:     "",
				})
			}
		}
	}
	return data
}

func loadDb() api.Db {
	file, err := ioutil.ReadFile(config.DbPath)
	if err != nil {
		fmt.Println("can not open file")
	}

	data := api.Db{}
	json.Unmarshal(file, &data)

	return data
}

func saveDb() {
	dataBytes, err := json.Marshal(db)
	if err != nil {
		fmt.Println("can not marshal data")
		return
	}

	err = ioutil.WriteFile(config.DbPath, dataBytes, 0644)
	if err != nil {
		fmt.Println("can not write to db")
		return
	}
}

func handler() http.Handler {

	getHandler := http.NewServeMux()
	getHandler.Handle("/dynamic/", dynamicHandler())
	getHandler.Handle("/static/", staticHandler())
	getHandler.Handle("/md5/", md5Handler())
	getHandler.Handle("/api/", api.ApiHandler(&db, &config))
	getHandler.Handle("/", web.WebContentHandler())

	postHandler := http.NewServeMux()
	postHandler.Handle("/new/", newGenesisHandler())
	postHandler.Handle("/hash/", checkHashHandler())

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// make sure to close the request when done
		defer func() {
			if err := req.Body.Close(); err != nil {
				fmt.Println("could not close request body")
			}
		}()

		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(req.Body)

		if req.Method == http.MethodPost {
			postHandler.ServeHTTP(rw, req)
			return
		} else if req.Method == http.MethodGet {
			getHandler.ServeHTTP(rw, req)
			return
		}
		// we do not accept anything else
		http.Error(rw, "forbidden request received", http.StatusForbidden)
	})
}

func serve() {
	s := &http.Server{
		Addr:           ":" + config.Port,
		Handler:        handler(),
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 8192,
	}
	s.ListenAndServe()
}

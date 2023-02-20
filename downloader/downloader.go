package downloader

import (
	"Fantom-Genesis-Generator/api"
	"Fantom-Genesis-Generator/logger"
	"Fantom-Genesis-Generator/web"
	"encoding/json"
	"io"
	"net/http"
	"os"
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
	Name     string `json:"name"`
}

type CheckServer struct {
	Category string
	Type     string
	Url      string
	Hash     string
	Name     string
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
var log logger.Logger

func Init(path string) {
	log = logger.New()

	configPath = path
	config = loadConfig()
	loadCheckServers()

	db.Lock()
	defer db.Unlock()

	db = loadDb()

	for _, c := range config.Categories {
		for i := range c.StaticFiles {
			c.StaticFiles[i].Static = true
		}
	}

	api.UpdateData(&db, &config, log)

	serve()
}

func loadCheckServers() {
	checkServers.Lock()
	defer checkServers.Unlock()

	for _, c := range config.Categories {
		for _, cs := range c.CompareServers {
			for _, f := range c.DynamicFiles {
				checkServers.CheckServers = append(checkServers.CheckServers, CheckServer{
					Category: c.Name,
					Type:     f.Type,
					Url:      cs.Url,
					Name:     cs.Name,
					Hash:     "",
				})
			}
		}
	}
}

func loadConfig() api.Config {
	file, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("can not open config file:", err)
	}

	data := api.Config{}

	err = json.Unmarshal(file, &data)
	if err != nil {
		log.Fatal("can not unmarshal config file:", err)
	}

	bufferSize = data.BufferSize
	return data
}

func loadDb() api.Db {
	file, err := os.ReadFile(config.DbPath)
	if err != nil {
		log.Fatal("can not open db file:", err)
	}

	data := api.Db{}

	err = json.Unmarshal(file, &data)
	if err != nil {
		log.Fatal("can not unmarshal db file:", err)
	}

	return data
}

func saveDb() {
	dataBytes, err := json.Marshal(db)
	if err != nil {
		log.Error("can not marshal db data:", err)
		return
	}

	err = os.WriteFile(config.DbPath, dataBytes, 0644)
	if err != nil {
		log.Error("can not write data to db:", err)
		return
	}
}

func backupDb() {
	dataBytes, err := json.Marshal(db)
	if err != nil {
		log.Error("can not marshal db backup data:", err)
		return
	}

	err = os.WriteFile(config.BackupDbPath, dataBytes, 0644)
	if err != nil {
		log.Error("can not write data to backup db:", err)
		return
	}
}

func handler() http.Handler {

	getHandler := http.NewServeMux()
	getHandler.Handle("/dynamic/", dynamicHandler())
	getHandler.Handle("/static/", staticHandler())
	getHandler.Handle("/md5/", md5Handler())
	getHandler.Handle("/api/", api.ApiHandler(log))
	getHandler.Handle("/", web.WebContentHandler(log))

	postHandler := http.NewServeMux()
	postHandler.Handle("/new/", newGenesisHandler())
	postHandler.Handle("/hash/", checkHashHandler())

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// make sure to close the request when done
		defer func() {
			if err := req.Body.Close(); err != nil {
				log.Error("could not close request body:", err)
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
	err := s.ListenAndServe()
	if err != nil {
		log.Fatal("can not start server:", err)
		return
	}
}

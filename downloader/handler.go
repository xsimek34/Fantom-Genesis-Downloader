package downloader

import (
	"Fantom-Genesis-Generator/api"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func dynamicHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		var url = req.URL.Path

		var filename string
		url = strings.Replace(url, "/dynamic/", "", 1)
		filename = url
		url = strings.Replace(url, ".g", "", 1)

		r := regexp.MustCompile(`-`)
		parsed := r.Split(url, 3)

		epochNumber, err := strconv.Atoi(parsed[1])
		if err != nil {
			fmt.Println("could not parse epoch number")
			rw.WriteHeader(404)
			return
		}

		db.Lock()
		defer db.Unlock()

		var genesisCategory = parsed[0]

		var latestUnit = -1
		for _, unitCategory := range db.UnitsCategories {
			if unitCategory.CategoryName == genesisCategory {
				for _, unit := range unitCategory.Units {
					if unit.Epoch == epochNumber {
						latestUnit = unit.Index
					}
				}
			}
		}

		var genesisType = parsed[2]
		var unitsPath string

		for _, category := range config.Categories {
			if category.Name == genesisCategory {
				unitsPath = category.UnitsPath
			}
		}

		if len(unitsPath) == 0 {
			fmt.Println("could not find category")
			rw.WriteHeader(404)
			return
		}

		if latestUnit == -1 {
			fmt.Println("could not unit with " + strconv.Itoa(epochNumber) + " epoch number")
			rw.WriteHeader(404)
			return
		}

		var units = api.GetUnitsArray(genesisType, latestUnit, unitsPath)
		sendResponse(rw, req, units, filename)
	})
}

func staticHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		var filename = strings.Replace(req.URL.Path, "/static/", "", 1)
		var units []string
		units = append(units, config.StaticFilesPath+filename)

		sendResponse(rw, req, units, filename)
	})
}

func md5Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var filename = strings.Replace(req.URL.Path, "/md5/", "", 1)
		var units []string
		units = append(units, config.Md5sPath+filename)

		sendResponse(rw, req, units, filename)
	})
}

func newGenesisHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var newUnit NewUnit

		b, err := io.ReadAll(req.Body)

		defer req.Body.Close()

		if err != nil {
			log.Fatalln(err)
		}

		json.Unmarshal(b, &newUnit)

		if newUnit.Password != config.Password {
			fmt.Println("bad password")
			return
		}
		db.Lock()

		for _, unitCategory := range db.UnitsCategories {
			if unitCategory.CategoryName == newUnit.Category {
				if len(unitCategory.Units) == newUnit.Index {
					unitCategory.Units = append(unitCategory.Units, &api.Unit{Index: newUnit.Index, Epoch: newUnit.Epoch})
				} else {
					fmt.Println("incorrect unit number")
				}
			}
		}
		saveDb()

		db.Unlock()

		rw.WriteHeader(200)
		for _, category := range config.Categories {
			if category.Name == newUnit.Category {
				for _, file := range category.DynamicFiles {
					go generateMD5(file.Type, newUnit.Index, newUnit.Epoch, newUnit.Category)
				}
			}
		}

		var done = "done"

		rw.Write([]byte(done))

		api.UpdateData(&db, &config)
	})
}

func checkHashHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var checkHash CheckHash

		b, err := io.ReadAll(req.Body)

		if err != nil {
			log.Fatalln(err)
		}

		json.Unmarshal(b, &checkHash)

		if checkHash.Password != config.Password {
			fmt.Println("bad password")
			return
		}

		checkServers.Lock()
		defer checkServers.Unlock()

		for i, server := range checkServers.CheckServers {
			if server.Category == checkHash.Category && checkHash.Type == server.Type && checkHash.Name == server.Name {
				checkServers.CheckServers[i].Hash = checkHash.Hash
				rw.WriteHeader(200)
				var done = "done"
				rw.Write([]byte(done))
				return
			}
		}
		fmt.Println("could not find specified server in list")
		rw.WriteHeader(404)
	})
}

func sendResponse(rw http.ResponseWriter, req *http.Request, units []string, filename string) {
	var unitsInfo []unit
	var contentLength int64
	for _, unitName := range units {
		fi, err := os.Stat(unitName)
		if err != nil {
			fmt.Println("could not open file")
			rw.WriteHeader(404)
			return
		}
		unitsInfo = append(unitsInfo, unit{name: unitName, size: int(fi.Size())})
		contentLength += fi.Size()
	}

	var done = req.Context().Done()

	if req.Header.Get("Range") == "" {
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Length", strconv.Itoa(int(contentLength)))
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		rw.Header().Set("accept-range", "bytes")
		rw.WriteHeader(200)

		for _, unit := range units {
			file, err := os.Open(unit)
			if err != nil {
				fmt.Println("could not open file")
				return
			}

			b1 := make([]byte, bufferSize)

			for true {
				select {
				case <-done:
					return
				default:
				}
				n1, err := file.Read(b1)
				if n1 == 0 {
					break
				}

				if err != nil {
					fmt.Println("could not read from file")
				}
				rw.Write(b1[:n1])
			}
		}
	} else {
		var from, to = getBytes(req.Header.Get("Range"))

		if to == 0 {
			to = int(contentLength - 1)
		}

		var byteRange = to - from + 1

		rw.Header().Set("Content-Type", "multipart/byteranges")
		rw.Header().Set("Content-Length", strconv.Itoa(byteRange))
		rw.Header().Set("Content-Range", "bytes "+strconv.Itoa(from)+"-"+strconv.Itoa(to)+"/"+strconv.Itoa(int(contentLength)))
		rw.WriteHeader(206)

		for _, unit := range unitsInfo {
			file, err := os.Open(unit.name)
			if err != nil {
				fmt.Println("could not open file")
				return
			}

			if from > unit.size {
				from = from - unit.size
				continue
			} else if from > 0 {
				_, err = file.Seek(int64(from), io.SeekStart)
				if err != nil {
					fmt.Println("could not read from file")
				}
				from = 0
			}

			b1 := make([]byte, bufferSize)

			for true {
				select {
				case <-done:
					return
				default:
				}

				if byteRange > bufferSize {
					n1, err := file.Read(b1)
					if n1 == 0 {
						break
					}
					if err != nil {
						fmt.Println("could not read from file")
					}
					written, err := rw.Write(b1[:n1])
					byteRange = byteRange - written
					if err != nil {
						log.Fatalln(err)
					}
				} else if byteRange == 0 {
					break
				} else {
					b2 := make([]byte, byteRange)
					n2, err := file.Read(b2)
					if n2 == 0 {
						break
					}
					if err != nil {
						fmt.Println("could not read from file")
					}
					rw.Write(b2[:n2])
					byteRange = 0
				}

			}
		}
	}
}

func getBytes(bytes string) (int, int) {
	bytes = strings.Split(bytes, "=")[1]

	var from, _ = strconv.Atoi(strings.Split(bytes, "-")[0])
	var to, _ = strconv.Atoi(strings.Split(bytes, "-")[1])

	return from, to
}

func generateMD5(genesisType string, latestUnit int, latestEpoch int, categoryName string) {
	// set category
	var category *api.Category
	for _, c := range config.Categories {
		if c.Name == categoryName {
			category = c
		}
	}

	var filename = config.Md5sPath + categoryName + "-" + strconv.Itoa(latestEpoch) + "-" + genesisType + ".g" + ".md5"
	// get hash from slices

	var units = api.GetUnitsArray(genesisType, latestUnit, "")
	hasher := md5.New()

	for _, unit := range units {

		file, err := os.Open(category.UnitsPath + unit)
		if err != nil {
			fmt.Println("could not open file")
			return
		}

		b1 := make([]byte, bufferSize)

		for true {
			n1, err := file.Read(b1)
			if n1 == 0 {
				break
			}

			if err != nil {
				fmt.Println("could not read from file")
			}

			hasher.Write(b1[:n1])
		}
	}

	// get hash from hasher
	var hash = hex.EncodeToString(hasher.Sum(nil))

	// send hash to servers
	checkServers.Lock()
	for _, server := range checkServers.CheckServers {
		if server.Category == categoryName && server.Type == genesisType {
			server.Hash = ""
			sendHashToServer(hash, server.Url, config.Password, categoryName, genesisType)
		}
	}
	checkServers.Unlock()

	// wait for check hashes from other servers
	for true {
		var validated = true
		checkServers.Lock()
		for _, server := range checkServers.CheckServers {
			if server.Category == categoryName && server.Type == genesisType {
				if server.Hash != hash {
					validated = false
				}
			}
		}
		checkServers.Unlock()
		if validated {
			break
		}
		time.Sleep(10 * time.Second)
	}

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		// after check generate md5 file
		f, err := os.Create(filename)
		if err != nil {
			fmt.Println("could not create file")
			return
		}
		f.Write([]byte(hex.EncodeToString(hasher.Sum(nil))))

		api.UpdateData(&db, &config)
		fmt.Println("Generating MD5 for " + categoryName + "-" + genesisType + " is done.")
	} else {
		fmt.Println("MD5 for " + categoryName + "-" + genesisType + " is already generated")
	}

}

func sendHashToServer(hash string, server string, password string, category string, genesisType string) {
	var hashRequest = CheckHash{
		Hash:     hash,
		Password: password,
		Name:     config.Name,
		Category: category,
		Type:     genesisType,
	}

	postBody, _ := json.Marshal(hashRequest)
	responseBody := bytes.NewBuffer(postBody)

	resp, err := http.Post(server+"hash/", "application/json", responseBody)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	log.Printf(sb)
}

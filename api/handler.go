package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type Config struct {
	Name            string      `json:"server_name"`
	Port            string      `json:"port"`
	DbPath          string      `json:"db_path"`
	Md5sPath        string      `json:"md5s_path"`
	StaticFilesPath string      `json:"static_files_path"`
	BufferSize      int         `json:"buffer_size"`
	Password        string      `json:"password"`
	Categories      []*Category `json:"categories"`
}

type Db struct {
	UnitsCategories []*UnitCategory `json:"units_categories"`
	sync.RWMutex
}

type Unit struct {
	Index int `json:"index"`
	Epoch int `json:"epoch"`
}

type UnitCategory struct {
	CategoryName string  `json:"category_name"`
	Units        []*Unit `json:"units"`
}

type Category struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	UnitsPath      string          `json:"units_path"`
	CompareServers []CompareServer `json:"compare_servers"`
	DynamicFiles   []*DynamicFile  `json:"dynamic_files"`
	StaticFiles    []*GenesisFile  `json:"static_files"`
}

type CompareServer struct {
	Name string `json:"server_name"`
	Url  string `json:"url"`
}

type DynamicFile struct {
	Type         string `json:"type"`
	Fullsync     bool   `json:"fullsync"`
	Snapsync     bool   `json:"snapsync"`
	BlockHistory string `json:"block_history"`
	EvmHistory   string `json:"evm_history"`
	Size         int    `json:"size"`
}

type GenesisFile struct {
	Category     string `json:"category"`
	Static       bool   `json:"static"`
	Name         string `json:"name"`
	Md5          string `json:"md5"`
	Epoch        int    `json:"epoch"`
	Block        int    `json:"block"`
	Fullsync     bool   `json:"fullsync"`
	Snapsync     bool   `json:"snapsync"`
	BlockHistory string `json:"block_history"`
	EvmHistory   string `json:"evm_history"`
	FileSize     int    `json:"file_size"`
}

type WebData struct {
	Categories   []*Category    `json:"categories"`
	GenesisFiles []*GenesisFile `json:"genesis_files"`
}

// DataLock defines a struct of prometheus data
type DataLock struct {
	data []byte
	sync.RWMutex
}

var dataLock DataLock

// ApiHandler handles the static content of the website
// using resources stored within the embedded FS.
func ApiHandler(db *Db, config *Config) http.Handler {

	//updateData(db, config) //TODO: change location of this call, every event

	// make the handler
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		rw.Header().Set("Content-Type", "application/json")

		dataLock.RLock()
		defer dataLock.RUnlock()

		if _, err := rw.Write(dataLock.data); err != nil {
			fmt.Errorf("could not write response; %s", err.Error())
		}
	})
}

func getLatestEpochAndIndex(db *Db, category string) (int, int) {
	var latestEpoch = -1
	var latestIndex = -1

	for _, unitCategory := range db.UnitsCategories {
		if unitCategory.CategoryName == category {
			for _, unit := range unitCategory.Units {
				if unit.Index > latestIndex {
					latestIndex = unit.Index
					latestEpoch = unit.Epoch
				}
			}
			return latestEpoch, latestIndex
		}
	}
	return latestEpoch, latestIndex
}

func UpdateData(db *Db, config *Config) {
	db.Lock()
	defer db.Unlock()

	var webData WebData

	webData.Categories = config.Categories

	for _, c := range config.Categories {
		for _, f := range c.StaticFiles {
			var file = f
			file.Category = c.Name
			webData.GenesisFiles = append(webData.GenesisFiles, file)
		}

		for _, f := range c.DynamicFiles {
			var genesis = &GenesisFile{
				Category:     c.Name,
				Static:       false,
				Fullsync:     f.Fullsync,
				Snapsync:     f.Snapsync,
				BlockHistory: f.BlockHistory,
				EvmHistory:   f.EvmHistory,
			}

			var latestEpoch, latestIndex = getLatestEpochAndIndex(db, c.Name)
			if latestEpoch == -1 {
				fmt.Println("could not get latest epoch number")
				return
			}
			genesis.Epoch = latestEpoch

			var filename = c.Name + "-" + strconv.Itoa(latestEpoch) + "-" + f.Type + ".g"
			genesis.Name = filename

			if _, err := os.Stat(config.Md5sPath + filename + ".md5"); err == nil {
				genesis.Md5 = filename + ".md5"
			} else if errors.Is(err, os.ErrNotExist) {
				genesis.Md5 = ""
			}

			genesis.FileSize = getFileSize(c.UnitsPath, f.Type, latestIndex)
			webData.GenesisFiles = append(webData.GenesisFiles, genesis)
		}
	}

	data, err := json.Marshal(webData)
	if err != nil {
		fmt.Errorf("could not marshal data; %s", err.Error())
		return
	}

	dataLock.data = data
}

func getFileSize(unitsPath string, genesisType string, latestIndex int) int {
	var units = GetUnitsArray(genesisType, latestIndex, unitsPath)
	var contentLength int64
	for _, unitName := range units {
		fi, err := os.Stat(unitName)
		if err != nil {
			fmt.Println("could not open file")
		}
		contentLength += fi.Size()
	}
	return int(contentLength)
}

func getUnits(unitType string, fromIndex int, toIndex int, unitsPath string) []string {
	var unitName = ""
	if unitType == "blocks" {
		unitName = "brs"
	} else if unitType == "epochs" {
		unitName = "ers"
	} else if unitType == "evm" {
		unitName = "evm"
	}

	var units []string

	if fromIndex == toIndex {
		units = append(units, unitsPath+unitType+"/"+unitName+"-"+strconv.Itoa(toIndex)+".g")
	} else {
		var i int
		for i = 0; i <= toIndex; i++ {
			units = append(units, unitsPath+unitType+"/"+unitName+"-"+strconv.Itoa(i)+".g")
		}
	}
	return units
}

func GetUnitsArray(genesisType string, latestUnit int, unitsPath string) []string {
	var units []string

	if genesisType == "full-mpt" {
		units = append(units, getUnits("blocks", 0, latestUnit, unitsPath)...)
		units = append(units, getUnits("epochs", 0, latestUnit, unitsPath)...)
		units = append(units, getUnits("evm", 0, latestUnit, unitsPath)...)
	} else if genesisType == "pruned-mpt" {
		units = append(units, getUnits("blocks", 0, latestUnit, unitsPath)...)
		units = append(units, getUnits("epochs", 0, latestUnit, unitsPath)...)
		units = append(units, getUnits("evm", latestUnit, latestUnit, unitsPath)...)
	} else if genesisType == "no-mpt" {
		units = append(units, getUnits("blocks", 0, latestUnit, unitsPath)...)
		units = append(units, getUnits("epochs", 0, latestUnit, unitsPath)...)
	}

	return units
}

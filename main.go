package main

import "Fantom-Genesis-Generator/downloader"

var configPath = "config.json"

func main() {
	downloader.Init(configPath)
}

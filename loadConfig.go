package main

import (
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"os"
	"path"
	"fmt"
)

func loadConfig(filenamePath *string, filename *string) {
	log := logging.MustGetLogger("log")

	viper.SetConfigName(*filename)
	viper.AddConfigPath(*filenamePath)

	if err := viper.ReadInConfig(); err != nil {
		log.Critical("Unable to load config \"" + path.Join(*filenamePath, *filename)+ "\" file:", err)
		os.Exit(1)
	}

	switch viper.GetString("logtype") {
	case "critical":
		logging.SetLevel(0, "")
	case "error":
		logging.SetLevel(1, "")
	case "warning":
		logging.SetLevel(2, "")
	case "notice":
		logging.SetLevel(3, "")
	case "info":
		logging.SetLevel(4, "")
	case "debug":
		logging.SetLevel(5, "")
		log.Debug("\"debug\" is selected")
	default:
		logging.SetLevel(2, "")
	}

	log.Debug("loadConfig func:")
	log.Debug(fmt.Sprintf("  path: %s", *filenamePath))
	log.Debug(fmt.Sprintf("  filename: %s", *filename))
	log.Debug(fmt.Sprintf("  logtype in file config is \"%s\"", viper.GetString("logtype")))
}

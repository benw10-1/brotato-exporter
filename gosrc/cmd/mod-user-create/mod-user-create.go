package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterstore"
	"github.com/benw10-1/brotato-exporter/exporterstore/exporterstoretypes"
	"github.com/google/uuid"
)

var (
	host           string
	port           int
	https          bool
	verifyHost     bool
	maxSubscribers int
)

func prompts() {
	var prompt survey.Prompt
	prompt = &survey.Input{
		Message: "Host",
		Default: "127.0.0.1",
	}
	err := survey.AskOne(prompt, &host)
	if err != nil {
		panic(err)
	}

	prompt = &survey.Input{
		Message: "Port",
		Default: "8081",
	}
	err = survey.AskOne(prompt, &port)
	if err != nil {
		panic(err)
	}

	prompt = &survey.Confirm{
		Message: "HTTPS",
		Default: false,
	}
	err = survey.AskOne(prompt, &https)
	if err != nil {
		panic(err)
	}

	prompt = &survey.Confirm{
		Message: "Verify Host",
		Default: false,
	}
	err = survey.AskOne(prompt, &verifyHost)
	if err != nil {
		panic(err)
	}

	prompt = &survey.Input{
		Message: "Max Subscribers",
		Default: "5",
	}
	err = survey.AskOne(prompt, &maxSubscribers)
	if err != nil {
		panic(err)
	}
}

func main() {
	prompts()
	exporterStore, err := exporterstore.NewExporterStore("/var/brotatoexporter/user.db")
	if err != nil {
		panic(err)
	}
	defer exporterStore.Close()

	authKeyRaw := make([]byte, 32)

	_, err = rand.Read(authKeyRaw)
	if err != nil {
		panic(err)
	}

	authKey := base64.StdEncoding.EncodeToString(authKeyRaw)

	user := &exporterstoretypes.ExporterUser{
		UserID:         uuid.New(),
		MaxSubscribers: maxSubscribers,
	}

	err = exporterStore.UpsertUser(user)
	if err != nil {
		panic(err)
	}

	err = exporterStore.UpsertAuthKeyUserID([]byte(authKey), user.UserID)
	if err != nil {
		panic(err)
	}

	config := brotatomodtypes.ModConfig{
		Enabled: true,
		ConnectionData: brotatomodtypes.ModConfigConnectionData{
			Host:       host,
			Port:       port,
			HTTPS:      https,
			VerifyHost: verifyHost,
			AuthToken:  authKey,
		},
	}

	makeConfigFile(config)
	makeUserModZip()

	log.Printf("User created with ID (%s) with config - %+v", user.UserID, config)
}

func makeUserModZip() {
	zipFile, err := os.Create("/var/brotatoexporter/user-mod.zip")
	if err != nil {
		panic(err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	baseDir := "/var/lib/mod"

	err = filepath.Walk(baseDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return errutil.NewStackError(err)
		}
		path = path[len(baseDir):]

		if path == "" {
			return nil
		}
		// remove leading slash
		path = path[1:]
		fmt.Println("Zipping -", path)

		if info.IsDir() {
			_, err = zipWriter.Create(fmt.Sprintf("%s%c", path, os.PathSeparator))

			return errutil.NewStackError(err)
		}

		file, err := os.Open(filepath.Join(baseDir, path))
		if err != nil {
			return errutil.NewStackError(err)
		}
		defer file.Close()

		f, err := zipWriter.Create(path)
		if err != nil {
			return errutil.NewStackError(err)
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return errutil.NewStackError(err)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func makeConfigFile(config brotatomodtypes.ModConfig) {
	err := os.MkdirAll("/var/brotatoexporter", 0755)
	if err != nil {
		panic(err)
	}

	outputConfigFile, err := os.Create("/var/brotatoexporter/connect-config.json")
	if err != nil {
		panic(err)
	}
	defer outputConfigFile.Close()

	zipTargetConfigFile, err := os.Create("/var/lib/mod/mods-unpacked/benw10-BrotatoExporter/connect-config.json")
	if err != nil {
		panic(err)
	}
	defer zipTargetConfigFile.Close()

	multiWriter := io.MultiWriter(outputConfigFile, zipTargetConfigFile)

	err = json.NewEncoder(multiWriter).Encode(config)
	if err != nil {
		panic(err)
	}
}

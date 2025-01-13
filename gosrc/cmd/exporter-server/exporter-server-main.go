package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterserver"
	"github.com/benw10-1/brotato-exporter/exporterserver/ctrlauth"
	"github.com/benw10-1/brotato-exporter/exporterserver/ctrlmessage"
	"github.com/benw10-1/brotato-exporter/exporterserver/messagesubhandler"
	"github.com/benw10-1/brotato-exporter/exporterstore"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

func loadYAMLConfig() error {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	viper.SetConfigName("default")

	viper.AddConfigPath("/etc/brotatoexporter")
	viper.AddConfigPath("/var/brotatoexporter")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		return errutil.NewStackError(err)
	}

	viper.SetConfigName("override")

	err = viper.MergeInConfig()
	if err != nil {
		_, isNotFound := err.(viper.ConfigFileNotFoundError)
		if !isNotFound {
			return errutil.NewStackError(err)
		}

		// generate base config override file - this is just the auth key for now so not importing the yaml writer stuff
		overrideFile, err := os.Create("/var/brotatoexporter/override.yaml")
		if err != nil {
			return errutil.NewStackError(err)
		}
		defer overrideFile.Close()

		authKeyRaw := make([]byte, 32)

		_, err = rand.Read(authKeyRaw)
		if err != nil {
			return errutil.NewStackError(err)
		}

		authKey := base64.StdEncoding.EncodeToString(authKeyRaw)

		_, err = overrideFile.WriteString(fmt.Sprintf(`jwt-auth-signing-key: "%s"`, authKey))
		if err != nil {
			return errutil.NewStackError(err)
		}

		viper.Set("jwt-auth-signing-key", authKey)

		log.Println("No override config file found, created one at /var/brotatoexporter/override.yaml")
	}

	return nil
}

func main() {
	appCtx, cancelAppCtx := context.WithCancel(context.Background())
	defer cancelAppCtx()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancelAppCtx()
	}()

	appLogWriter := &lumberjack.Logger{
		Filename:   "/var/log/exporter-server-app.log",
		MaxSize:    50,
		MaxBackups: 3,
		Compress:   true,
	}
	log.SetOutput(appLogWriter)

	requestLogWriter := &lumberjack.Logger{
		Filename:   "/var/log/exporter-server-requests.log",
		MaxSize:    50,
		MaxBackups: 3,
		Compress:   true,
	}
	requestLogger := log.New(requestLogWriter, "", 0)

	go func() {
		err := http.ListenAndServe(viper.GetString("pprof-serve-addr"), nil)
		if err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	err := loadYAMLConfig()
	if err != nil {
		panic(err)
	}

	exporterStore, err := exporterstore.NewExporterStore("/var/brotatoexporter/user.db")
	if err != nil {
		panic(err)
	}
	defer exporterStore.Close()

	sessionInfoMap := new(ctrlauth.SessionInfoMap)

	handlerList := make([]http.Handler, 0)

	authAPI := ctrlauth.NewAuthAPI([]byte(viper.GetString("jwt-auth-signing-key")), sessionInfoMap, exporterStore)
	handlerList = append(handlerList, authAPI)

	subHandler := messagesubhandler.NewMessageSubHandler(appCtx, sessionInfoMap, time.Minute*10)

	messageAPI := ctrlmessage.NewMessageAPI(sessionInfoMap, exporterStore, subHandler)
	handlerList = append(handlerList, messageAPI)

	exporterServer := exporterserver.NewExporterServer(handlerList, requestLogger)

	srv := http.Server{
		Addr:         viper.GetString("serve-addr"),
		Handler:      exporterServer,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	go func() {
		<-appCtx.Done()
		shutdownCtx, cancelShutdownCtx := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdownCtx()

		err = srv.Shutdown(shutdownCtx)
		log.Printf("Server shutdown with err: %v", err)
	}()

	log.Printf("Server listening on %s", srv.Addr)

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server error: %v", err)
	}
}

package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rf152/streamdeck-tricks/addons"
	streamdeck "github.com/rf152/go-streamdeck"
	_ "github.com/rf152/go-streamdeck/devices"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var sd *streamdeck.StreamDeck

func loadConfigAndDefaults() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04"})

	// first set some default values
	viper.AddConfigPath(".")
	viper.SetDefault("buttons.images", "images/buttons") // location of button images
	viper.SetDefault("obs.host", "localhost")            // OBS webhooks endpoint
	viper.SetDefault("obs.port", 4444)                   // OBS webhooks endpoint
	viper.SetDefault("obs.password", "")                   // OBS webhooks endpoint
	viper.SetDefault("mqtt.uri", "tcp://10.1.0.1:1883")  // MQTT server location

	// now read in config for any overrides
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		log.Warn().Msgf("Cannot read config file: %s \n", err)
	}
}

func main() {
	loadConfigAndDefaults()
	log.Info().Msg("Starting streamdeck tricks. Hai!")
	streamdeckfound := false
	var err error
	while(!streamdeckfound) {
		sd, err = streamdeck.New()
		if err != nil {
			log.Error().Err(err).Msg("Error finding Streamdeck")
			time.sleep(2 * time.Second)
		}
	}

	// init OBS
	// Initialise OBS to use OBS features (requires websockets plugin in OBS)
	obs_addon := addons.Obs{SD: sd}
	obs_addon.Init()
	while(!obs_addon.connected()) {
		time.sleep(2 * time.Second)
		obs_addon.connect()
	}
	obs_addon.Buttons()

	go webserver()

	log.Info().Msg("Up and running")
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}

func webserver() {
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})

	http.ListenAndServe(":7001", nil)
}

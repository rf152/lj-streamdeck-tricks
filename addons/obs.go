package addons

import (
	"image/color"
	"strings"

	obsws "github.com/christopher-dG/go-obs-websocket"
	"github.com/rf152/go-streamdeck"
	"github.com/rf152/go-streamdeck/buttons"
	sddecorators "github.com/rf152/go-streamdeck/decorators"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Obs struct {
	SD         *streamdeck.StreamDeck
	obs_client obsws.Client
	Offset     int
}

var obs_program_scene string
var obs_preview_scene string
var streaming bool

type ObsScene struct {
	Name     string `mapstructure:"name"`
	Image    string `mapstructure:"image"`
	Button   int    `mapstructure:"button"`
	ButtonId int
}

func (scene *ObsScene) SetButtonId(id int) {
	scene.ButtonId = id
}

var buttons_obs map[string]*ObsScene // scene name and image name

func (o *Obs) Init() {
	streaming = false
	o.connect()
}

func (o *Obs) connect() {
	log.Debug().Msg("Connecting to OBS...")
	log.Info().Msgf("%#v\n", viper.Get("obs.host"))
	o.obs_client = obsws.Client{Host: viper.Get("obs.host"), Port: viper.Get("obs.port"), Password: viper.Get("obs.password")}
	err := o.obs_client.Connect()
	if err != nil {
		log.Warn().Err(err).Msg("Cannot connect to OBS")
	}
	if (o.connected()) {
		o.ObsEventHandlers()
	}
}

func (o *Obs) connected() {
	return o.obs_client.Connected() == true
}

func (o *Obs) ObsEventHandlers() {
	if o.obs_client.Connected() == true {
		// PGM change
		o.obs_client.AddEventHandler("SwitchScenes", func(e obsws.Event) {
			// Make sure to assert the actual event type.
			scene := strings.ToLower(e.(obsws.SwitchScenesEvent).SceneName)
			log.Info().Msg("Old scene: " + obs_program_scene)
			// undecorate the old
			if obs_program_scene != obs_preview_scene {
				if oldb, ok := buttons_obs[obs_program_scene]; ok {
					log.Info().Int("button", oldb.ButtonId).Msg("Clear original button decoration")
					o.SD.UnsetDecorator(oldb.ButtonId)
				}
			} else {
				if eventb, ok := buttons_obs[obs_preview_scene]; ok {
					decorator2 := sddecorators.NewBorder(5, color.RGBA{0, 255, 0, 255})
					log.Info().Int("button", eventb.ButtonId).Msg("Highlight new scene button")
					o.SD.SetDecorator(eventb.ButtonId, decorator2)
				}
			}
			// decorate the new
			log.Info().Msg("New scene: " + scene)
			if eventb, ok := buttons_obs[scene]; ok {
				decorator2 := sddecorators.NewBorder(5, color.RGBA{255, 0, 0, 255})
				log.Info().Int("button", eventb.ButtonId).Msg("Highlight new scene button")
				o.SD.SetDecorator(eventb.ButtonId, decorator2)
			}
			obs_program_scene = scene
		})

		// PVW change
		o.obs_client.AddEventHandler("PreviewSceneChanged", func(e obsws.Event) {
			// Make sure to assert the actual event type.
			scene := strings.ToLower(e.(obsws.PreviewSceneChangedEvent).SceneName)
			log.Info().Msg("Old scene: " + obs_preview_scene)
			// undecorate the old
			if obs_preview_scene != obs_program_scene {
				if oldb, ok := buttons_obs[obs_preview_scene]; ok {
					log.Info().Int("button", oldb.ButtonId).Msg("Clear original button decoration")
					o.SD.UnsetDecorator(oldb.ButtonId)
				}
			}
			// decorate the new
			log.Info().Msg("New scene: " + scene)
			if scene != obs_program_scene {
				if eventb, ok := buttons_obs[scene]; ok {
					decorator2 := sddecorators.NewBorder(5, color.RGBA{0, 255, 0, 255})
					log.Info().Int("button", eventb.ButtonId).Msg("Highlight new scene button")
					o.SD.SetDecorator(eventb.ButtonId, decorator2)
				}
			}
			obs_preview_scene = scene
		})

		// Streaming State change
		o.obs_client.AddEventHandler("StreamStarted", func(e obsws.Event) {
			image_path := viper.GetString("buttons.images")
			// Set the button style
			log.Info().Msg("Started stream")
			
			streambutton, err := buttons.NewImageFileButton(image_path + "/cloud-upload-green.png")
			if err == nil {
				streambutton.SetActionHandler(&OBSToggleStreamAction{Client: o.obs_client, Obs: o})
				o.SD.AddButton(14, streambutton)
			} else {
				log.Warn().Err(err)
			}
		})

		// Streaming State change
		o.obs_client.AddEventHandler("StreamStopped", func(e obsws.Event) {
			image_path := viper.GetString("buttons.images")
			// Set the button style
			log.Info().Msg("Stopped stream")
			
			streambutton, err := buttons.NewImageFileButton(image_path + "/cloud-upload-red.png")
			if err == nil {
				streambutton.SetActionHandler(&OBSToggleStreamAction{Client: o.obs_client, Obs: o})
				o.SD.AddButton(14, streambutton)
			} else {
				log.Warn().Err(err)
			}
		})

		// OBS Exits
		o.obs_client.AddEventHandler("Exiting", func(e obsws.Event) {
			log.Info().Msg("OBS has exited")
			o.ClearButtons()
		})

		// Scene Collection Switched
		o.obs_client.AddEventHandler("SceneCollectionChanged", func(e obsws.Event) {
			log.Info().Msg("Scene collection changed")
			o.ClearButtons()
			o.Buttons()
		})

	}
}

func (o *Obs) Buttons() {
	if o.obs_client.Connected() == true {
		// OBS Scenes to Buttons
		buttons_obs = make(map[string]*ObsScene)
		viper.UnmarshalKey("obs_scenes", &buttons_obs)
		image_path := viper.GetString("buttons.images")
		var image string
		var button int

		// what scenes do we have? (max 8 for the top row of buttons)
		scene_req := obsws.NewGetSceneListRequest()
		scenes, err := scene_req.SendReceive(o.obs_client)
		if err != nil {
			log.Warn().Err(err)
		}
		obs_program_scene = strings.ToLower(scenes.CurrentScene)

		preview_req := obsws.NewGetPreviewSceneRequest()
		preview, err := preview_req.SendReceive(o.obs_client)
		if err != nil {
			log.Warn().Err(err)
		}
		obs_preview_scene := strings.ToLower(preview.Name)

		// make buttons for these scenes
		for _, scene := range scenes.Scenes {
			log.Debug().Msg("Scene: " + scene.Name)
			image = ""
			button = 0
			oaction := &OBSSceneAction{Scene: scene.Name, Client: o.obs_client}
			sceneName := strings.ToLower(scene.Name)

			if s, ok := buttons_obs[sceneName]; ok {
				if s.Image != "" {
					image = image_path + s.Image
				}
				button = s.Button
			} else {
				// there wasn't an entry in the buttons for this scene so add one
				buttons_obs[sceneName] = &ObsScene{}
			}

			if image != "" {
				// try to make an image button

				obutton, err := buttons.NewImageFileButton(image)
				if err == nil {
					obutton.SetActionHandler(oaction)
					o.SD.AddButton(button, obutton)
					// store which button we just set
					buttons_obs[sceneName].SetButtonId(button)
				} else {
					// something went wrong with the image, use a default one
					image = image_path + "/play.jpg"
					obutton, err := buttons.NewImageFileButton(image)
					if err == nil {
						obutton.SetActionHandler(oaction)
						o.SD.AddButton(button, obutton)
						// store which button we just set
						buttons_obs[sceneName].SetButtonId(button)
					}
				}
			} else {
				log.Info().Msg("Text button")
				// use a text button
				oopbutton := buttons.NewTextButton(scene.Name)
				oopbutton.SetActionHandler(oaction)
				o.SD.AddButton(button, oopbutton)
				// store which button we just set
				buttons_obs[sceneName].SetButtonId(button)
			}
		}

		// Make the other buttons
		log.Debug().Msg("Creating swap button")
		swapbutton, err := buttons.NewImageFileButton(image_path + "/swap-horizontal-bold.png")
		if err == nil {
			swapbutton.SetActionHandler(&OBSSwitchAction{Client: o.obs_client, Obs: o})
			o.SD.AddButton(4, swapbutton)
		} else {
			log.Warn().Err(err)
		}

		log.Debug().Msg("Creating stream button")
		streambutton, err := buttons.NewImageFileButton(image_path + "/cloud-upload-red.png")
		if err == nil {
			streambutton.SetActionHandler(&OBSToggleStreamAction{Client: o.obs_client, Obs: o})
			o.SD.AddButton(14, streambutton)
		} else {
			log.Warn().Err(err)
		}

		// highlight the active scene
		if eventb, ok := buttons_obs[obs_preview_scene]; ok {
			decorator2 := sddecorators.NewBorder(5, color.RGBA{0, 255, 0, 255})
			log.Info().Int("button", eventb.ButtonId).Msg("Highlight current scene")
			o.SD.SetDecorator(eventb.ButtonId, decorator2)
		}
		// highlight the active scene
		if eventb, ok := buttons_obs[obs_program_scene]; ok {
			decorator2 := sddecorators.NewBorder(5, color.RGBA{255, 0, 0, 255})
			log.Info().Int("button", eventb.ButtonId).Msg("Highlight current scene")
			o.SD.SetDecorator(eventb.ButtonId, decorator2)
		}
	}
}

func (o *Obs) ClearButtons() {
	for i := 0; i < 7; i++ {
		o.SD.UnsetDecorator(o.Offset + i)
		clearbutton := buttons.NewTextButton("")
		o.SD.AddButton(o.Offset+i, clearbutton)
	}
}

type OBSSceneAction struct {
	Client obsws.Client
	Scene  string
	btn    streamdeck.Button
}

func (action *OBSSceneAction) Pressed(btn streamdeck.Button) {

	log.Info().Msg("Set scene: " + action.Scene)
	req := obsws.NewSetPreviewSceneRequest(action.Scene)
	_, err := req.SendReceive(action.Client)
	if err != nil {
		log.Warn().Err(err).Msg("OBS scene action error")
	}

}

type OBSSwitchAction struct {
	Client obsws.Client
	Obs    *Obs
	btn    streamdeck.Button
}

func (action *OBSSwitchAction) Pressed(btn streamdeck.Button) {
	log.Debug().Msg("Swap Scenes")
	req := obsws.NewTransitionToProgramRequest(make(map[string]interface{}), "fade", 300)
	_, err := req.SendReceive(action.Client)
	if err != nil {
		log.Warn().Err(err).Msg("OBS transition action error")
	}
}


type OBSToggleStreamAction struct {
	Client obsws.Client
	Obs    *Obs
	btn    streamdeck.Button
}

func (action *OBSToggleStreamAction) Pressed(btn streamdeck.Button) {
	log.Debug().Msg("Swap Scenes")
	req := obsws.NewStartStopStreamingRequest()
	_, err := req.SendReceive(action.Client)
	if err != nil {
		log.Warn().Err(err).Msg("OBS stream action error")
	}
}


type OBSStartAction struct {
	Client obsws.Client
	Obs    *Obs
	btn    streamdeck.Button
}

func (action *OBSStartAction) Pressed(btn streamdeck.Button) {
	log.Debug().Msg("Reinit OBS")
	if !action.Obs.obs_client.Connected() {
		action.Obs.Init()
	}
	action.Obs.ClearButtons()
	action.Obs.Buttons()
}

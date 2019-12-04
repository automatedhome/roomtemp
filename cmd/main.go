package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	mqttclient "github.com/automatedhome/common/pkg/mqttclient"
	types "github.com/automatedhome/roomtemp/pkg/types"
	scheduler "github.com/automatedhome/scheduler/pkg/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	config        types.Config
	settings      types.Settings
	actuators     types.Actuators
	schedule      scheduler.Schedule
	overrideEnd   time.Time
	mode          string
	scheduleTopic string
	client        mqtt.Client
)

func onMessage(client mqtt.Client, message mqtt.Message) {
	switch message.Topic() {
	case settings.Holiday.Address:
		value, err := strconv.ParseBool(string(message.Payload()))
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		settings.Holiday.Value = value
		if value {
			log.Println("We are in holiday mode!")
		}
		log.Println("Working days mode activated.")

	case settings.Override.Address:
		overrideEnd = time.Now().Add(time.Duration(60 * time.Minute))
		value, err := strconv.ParseFloat(string(message.Payload()), 64)
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		log.Printf("Overriding expected temperature to: '%f'\n", value)
		settings.Override.Value = value

	case settings.Mode.Address:
		value := string(message.Payload())
		setMode(value)

	case scheduleTopic:
		var tmp = scheduler.Schedule{}
		err := json.Unmarshal(message.Payload(), &tmp)
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		log.Printf("New schedule received: %+v", tmp)
		schedule = tmp
	}
}

func setMode(val string) {
	if val != "auto" && val != "heat" {
		return
	}
	if val == mode {
		return
	}
	mode = val

	log.Printf("Setting operation mode to: '%s'\n", mode)
	client.Publish(settings.Mode.Address, 0, false, mode)
	if mode == "heat" {
		overrideEnd = time.Now().Add(time.Duration(60 * time.Minute))
	}
	if mode == "auto" {
		overrideEnd = time.Now()
	}
}

func stringToDate(str string) time.Time {
	now := time.Now()
	t := strings.Split(str, ":")
	h, _ := strconv.Atoi(t[0])
	m, _ := strconv.Atoi(t[1])
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
}

func setExpected(value float64) {
	// Value is retained and persists in broker db
	if actuators.Expected.Value != value {
		client.Publish(actuators.Expected.Address, 0, true, fmt.Sprintf("%.2f", value))
		actuators.Expected.Value = value
		log.Printf("Setting expected temperature to %.2f", value)
	}
}

func init() {
	broker := flag.String("broker", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	clientID := flag.String("clientid", "thermostat", "A clientid for the connection")
	configFile := flag.String("config", "/config.yaml", "Provide configuration file with MQTT topic mappings")
	flag.Parse()

	brokerURL, err := url.Parse(*broker)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Reading configuration from %s", *configFile)
	data, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("File reading error: %v", err)
		return
	}

	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		log.Fatalf("error: %v", err)
	}

	log.Printf("Starting with following config: %+v", config)

	scheduleTopic = config.Schedule
	actuators = config.Actuators
	settings = config.Settings
	schedule.DefaultTemperature = 0

	var topics []string
	topics = append(topics, settings.Holiday.Address, settings.Override.Address, settings.Mode.Address, scheduleTopic)
	client = mqttclient.New(*clientID, brokerURL, topics, onMessage)
	log.Printf("Connected to %s as %s and waiting for messages\n", *broker, *clientID)

	setMode("auto")

	// Wait for settings data
	for {
		if schedule.DefaultTemperature != 0 {
			break
		}
		log.Println("Waiting 15s for schedule data...")
		time.Sleep(15 * time.Second)
	}
	log.Printf("Starting with schedule received: %+v\n", schedule)
}

func main() {
	// run program
	for {
		time.Sleep(1 * time.Second)

		if time.Now().Before(overrideEnd) {
			setMode("heat")
		} else {
			setMode("auto")
		}

		// check if manual override heating mode is enabled
		if mode == "heat" {
			setExpected(settings.Override.Value)
			continue
		}

		// check if now is the time to start heating
		cells := &schedule.Workday
		if settings.Holiday.Value {
			cells = &schedule.Freeday
		}

		temp := schedule.DefaultTemperature
		for _, cell := range *cells {
			from := stringToDate(cell.From)
			to := stringToDate(cell.To)
			if time.Now().After(from) && time.Now().Before(to) {
				temp = cell.Temperature
				continue
			}
		}

		setExpected(temp)
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	mqttclient "github.com/automatedhome/common/pkg/mqttclient"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Schedule struct {
	Workday []struct {
		From        string  `json:"from"`
		To          string  `json:"to"`
		Temperature float64 `json:"temperature"`
	} `json:"workday"`
	Freeday []struct {
		From        string  `json:"from"`
		To          string  `json:"to"`
		Temperature float64 `json:"temperature"`
	} `json:"freeday"`
	DefaultTemperature float64 `json:"defaultTemperature"`
}

type BoolPoint struct {
	v    bool
	addr string
}

type DataPoint struct {
	v    float64
	addr string
}

type Sensors struct {
	holiday  BoolPoint
	override DataPoint
}

type Actuators struct {
	expected DataPoint
}

var sensors Sensors
var actuators Actuators
var schedule Schedule
var overrideEnd time.Time
var scheduleTopic string
var client mqtt.Client

func onMessage(client mqtt.Client, message mqtt.Message) {
	switch message.Topic() {
	case sensors.holiday.addr:
		value, err := strconv.ParseBool(string(message.Payload()))
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		sensors.holiday.v = value

	case sensors.override.addr:
		overrideEnd = time.Now().Add(time.Duration(60 * time.Minute))
		value, err := strconv.ParseFloat(string(message.Payload()), 64)
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		sensors.override.v = value

	case scheduleTopic:
		var tmp = Schedule{}
		err := json.Unmarshal(message.Payload(), &tmp)
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		log.Printf("New schedule received: %+v", tmp)
		schedule = tmp
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
	if actuators.expected.v != value {
		client.Publish(actuators.expected.addr, 0, false, fmt.Sprintf("%.2f", value))
		actuators.expected.v = value
		log.Printf("Setting expected temperature to %.2f", value)
	}
}

func init() {
	sensors.holiday = BoolPoint{false, "heater/settings/holiday"}
	sensors.override = DataPoint{18, "heater/settings/override"}
	actuators.expected = DataPoint{18, "heater/settings/expected"}
	schedule.DefaultTemperature = 0
	scheduleTopic = "heater/settings/schedule"

	overrideEnd = time.Now()
}

func main() {
	broker := flag.String("broker", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	clientID := flag.String("clientid", "roomtemp", "A clientid for the connection")
	flag.Parse()

	brokerURL, _ := url.Parse(*broker)

	log.Printf("%s\n", sensors.holiday.addr)

	var topics []string
	topics = append(topics, sensors.holiday.addr, sensors.override.addr, scheduleTopic)
	client = mqttclient.New(*clientID, brokerURL, topics, onMessage)
	log.Printf("Connected to %s as %s and waiting for messages\n", *broker, *clientID)

	// Wait for sensors data
	for {
		if schedule.DefaultTemperature != 0 {
			break
		}
		log.Println("Waiting 15s for schedule data...")
		time.Sleep(15 * time.Second)
	}
	log.Printf("Starting with schedule received: %+v\n", schedule)

	// run program
	for {
		time.Sleep(1 * time.Second)

		// check if manual override heating mode is enabled
		if time.Now().Before(overrideEnd) {
			setExpected(sensors.override.v)
			continue
		}

		// check if now is the time to start heating
		cells := &schedule.Workday
		if sensors.holiday.v {
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

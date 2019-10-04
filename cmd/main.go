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
	common "github.com/automatedhome/common/pkg/types"
	scheduler "github.com/automatedhome/scheduler/pkg/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Sensors struct {
	holiday  common.BoolPoint
	override common.DataPoint
}

type Actuators struct {
	expected common.DataPoint
}

var sensors Sensors
var actuators Actuators
var schedule scheduler.Schedule
var overrideEnd time.Time
var scheduleTopic string
var client mqtt.Client

func onMessage(client mqtt.Client, message mqtt.Message) {
	switch message.Topic() {
	case sensors.holiday.Address:
		value, err := strconv.ParseBool(string(message.Payload()))
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		sensors.holiday.Value = value
		if value {
			log.Println("We are in holiday mode!")
		}
		log.Println("Working days mode activated.")

	case sensors.override.Address:
		overrideEnd = time.Now().Add(time.Duration(60 * time.Minute))
		value, err := strconv.ParseFloat(string(message.Payload()), 64)
		if err != nil {
			log.Printf("Received incorrect message payload: '%v'\n", message.Payload())
			return
		}
		log.Printf("Overriding expected temperature to: '%f'\n", value)
		sensors.override.Value = value

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

func stringToDate(str string) time.Time {
	now := time.Now()
	t := strings.Split(str, ":")
	h, _ := strconv.Atoi(t[0])
	m, _ := strconv.Atoi(t[1])
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
}

func setExpected(value float64) {
	// Value is retained and persists in broker db
	if actuators.expected.Value != value {
		client.Publish(actuators.expected.Address, 0, true, fmt.Sprintf("%.2f", value))
		actuators.expected.Value = value
		log.Printf("Setting expected temperature to %.2f", value)
	}
}

func init() {
	sensors.holiday = common.BoolPoint{Value: false, Address: "heater/settings/holiday"}
	sensors.override = common.DataPoint{Value: 18, Address: "heater/settings/override"}
	actuators.expected = common.DataPoint{Value: 18, Address: "heater/settings/expected"}
	schedule.DefaultTemperature = 0
	scheduleTopic = "heater/settings/schedule"

	overrideEnd = time.Now()
}

func main() {
	broker := flag.String("broker", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	clientID := flag.String("clientid", "roomtemp", "A clientid for the connection")
	flag.Parse()

	brokerURL, err := url.Parse(*broker)
	if err != nil {
		log.Fatal(err)
	}

	var topics []string
	topics = append(topics, sensors.holiday.Address, sensors.override.Address, scheduleTopic)
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
			setExpected(sensors.override.Value)
			continue
		}

		// check if now is the time to start heating
		cells := &schedule.Workday
		if sensors.holiday.Value {
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

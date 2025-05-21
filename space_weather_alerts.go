// space_weather_alerts.go
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

const alertCacheFile = ".swpc-alert-cache.json"

// Config holds runtime configuration values
type Config struct {
	TwilioSID      string  `json:"twilio_sid"`
	TwilioAuth     string  `json:"twilio_auth"`
	TwilioFrom     string  `json:"twilio_from"`
	TwilioTo       string  `json:"twilio_to"`
	DryRun         bool    `json:"dry_run"`
	CheckInterval  int     `json:"check_interval_minutes"`
	KpThreshold    float64 `json:"kp_threshold"`
	BzThreshold    float64 `json:"bz_threshold"`
	ProtonFluxThreshold float64 `json:"proton_flux_threshold"`
	XrayFluxThreshold float64 `json:"xray_flux_threshold"`
}

var config Config

func loadConfig() {
	defaultPath := os.ExpandEnv("$HOME/.config/swpc-alerts/config.json")
	data, err := ioutil.ReadFile(defaultPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
}

// Re-declare necessary types and utility functions

type Alert struct {
	Message string `json:"message"`
}

type KpIndex struct {
	TimeTag string  `json:"time_tag"`
	Kp      float64 `json:"kp_index"`
}

type BzReading struct {
	Bz float64 `json:"bz_gsm"`
	TimeTag string `json:"time_tag"`
}

type FluxReading struct {
	Energy string  `json:"energy"`
	Flux   float64 `json:"flux"`
	TimeTag string `json:"time_tag"`
}

type AlertCache map[string]bool

func loadAlertCache() AlertCache {
	cache := make(AlertCache)
	data, err := ioutil.ReadFile(alertCacheFile)
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}

func saveAlertCache(cache AlertCache) {
	data, _ := json.Marshal(cache)
	_ = ioutil.WriteFile(alertCacheFile, data, 0644)
}

func hashAlert(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

func sendSMS(body string) error {
	if config.DryRun {
		log.Println("[Dry Run] SMS would be sent:", body)
		return nil
	}

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: config.TwilioSID,
		Password: config.TwilioAuth,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(config.TwilioTo)
	params.SetFrom(config.TwilioFrom)
	params.SetBody(body)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		log.Printf("Twilio error: %v", err)
	} else {
		log.Printf("Twilio message sent. SID: %s", *resp.Sid)
	}
	return err
}

func fetchJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func processSWPCAlerts(cache AlertCache) {
	var alerts []Alert
	err := fetchJSON("https://services.swpc.noaa.gov/json/alerts.json", &alerts)
	if err != nil {
		log.Println("Error fetching SWPC alerts:", err)
		return
	}
	for _, alert := range alerts {
		msg := alert.Message
		if strings.Contains(msg, "G3") || strings.Contains(msg, "G4") || strings.Contains(msg, "G5") ||
			strings.Contains(msg, "S3") || strings.Contains(msg, "S4") || strings.Contains(msg, "S5") ||
			strings.Contains(msg, "R3") || strings.Contains(msg, "R4") || strings.Contains(msg, "R5") {

			hash := hashAlert(msg)
			if !cache[hash] {
				cache[hash] = true
				text := fmt.Sprintf("🌐 SWPC Alert: %s", msg)
				if err := sendSMS(text); err != nil {
					log.Println("SMS failed:", err)
				}
			}
		}
	}
}

func processKpIndex(cache AlertCache) {
	var kpList []KpIndex
	err := fetchJSON("https://services.swpc.noaa.gov/json/planetary_k_index_1m.json", &kpList)
	if err != nil || len(kpList) == 0 {
		log.Println("Error fetching Kp index:", err)
		return
	}
	latest := kpList[len(kpList)-1]
	if latest.Kp >= config.KpThreshold {
		msg := fmt.Sprintf("🧠 K-index Alert: Kp = %.2f at %s\nLinked to sleep disruption, anxiety, and focus issues.", latest.Kp, latest.TimeTag)
		hash := hashAlert(msg)
		if !cache[hash] {
			cache[hash] = true
			_ = sendSMS(msg)
		}
	}
}

func processBzField(cache AlertCache) {
	var bzList []BzReading
	err := fetchJSON("https://services.swpc.noaa.gov/products/summary/dscovr-solar-wind.json", &bzList)
	if err != nil || len(bzList) == 0 {
		log.Println("Error fetching Bz field:", err)
		return
	}
	latest := bzList[len(bzList)-1]
	if latest.Bz < config.BzThreshold {
		msg := fmt.Sprintf("🧠 Geomagnetic Instability Alert: Bz = %.2f nT at %s\nMay disrupt sleep, mood, or focus in sensitive individuals.", latest.Bz, latest.TimeTag)
		hash := hashAlert(msg)
		if !cache[hash] {
			cache[hash] = true
			_ = sendSMS(msg)
		}
	}
}

func main() {
	loadConfig()

	if len(os.Args) > 1 && os.Args[1] == "--test" {
		log.Println("Running in test mode – sending test SMS...")
		testMessage := "🚨 Test Alert: Space weather alert system is operational."
		err := sendSMS(testMessage)
		if err != nil {
			log.Fatalf("Failed to send test SMS: %v", err)
		} else {
			log.Println("Test SMS sent successfully.")
		}
		return
	}

	cache := loadAlertCache()
	log.Println("Starting space weather alert monitor...")
	if config.DryRun {
		log.Println("Running in dry-run mode. No SMS will be sent.")
	}
	for {
		processSWPCAlerts(cache)
		processKpIndex(cache)
		processBzField(cache)
		saveAlertCache(cache)
		time.Sleep(time.Duration(config.CheckInterval) * time.Minute)
	}
}

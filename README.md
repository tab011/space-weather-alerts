# Space Weather Alerts 🌐🧠

A lightweight Go-based alerting system that fetches space weather data and sends SMS alerts using Twilio.

## 📦 Features

- Monitors:
  - NOAA SWPC alerts (G, S, R scales)
  - Planetary K-index
  - Bz field for geomagnetic disruptions
- Sends SMS alerts via Twilio (configurable thresholds)
- Supports test/dry-run mode
- Systemd-capable for running in background

## 🔧 Requirements

- Go 1.18+
- Twilio account with messaging number
- Verified Toll-Free or A2P-registered 10DLC number
- `config.json` file in: `$HOME/.config/swpc-alerts/`

## 📝 Example `config.json`

```json
{
  "twilio_sid": "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "twilio_auth": "your_auth_token",
  "twilio_from": "+18885551234",
  "twilio_to": "+1yourphonenumber",
  "dry_run": false,
  "check_interval_minutes": 15,
  "kp_threshold": 7.0,
  "bz_threshold": -8.0,
  "proton_flux_threshold": 0.1,
  "xray_flux_threshold": 0.0001
}


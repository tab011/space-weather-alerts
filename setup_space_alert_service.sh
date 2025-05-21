#!/bin/bash
set -e

read -rp "Enter service account name (default: alertsvc): " svcuser
svcuser=${svcuser:-alertsvc}

# Determine user home or create service account if missing
if ! id "$svcuser" &>/dev/null; then
  svchome="/var/lib/${svcuser}"
  echo "Creating user $svcuser with home $svchome..."
  sudo useradd -r -m -d "$svchome" -s /usr/sbin/nologin "$svcuser"
  sudo passwd -l "$svcuser"
  echo "User created and expired login."
else
  svchome=$(eval echo "~$svcuser")
  echo "Using existing user $svcuser with home $svchome"
fi

# Fetch vars from service user's .bashrc
bashrc="$svchome/.bashrc"
required_vars=(TWILIO_SID TWILIO_TOKEN TWILIO_FROM TWIML_URL OPENAI_API_KEY)
declare -A env_vars

for var in "${required_vars[@]}"; do
  value=$(sudo grep "export $var=" "$bashrc" | sed -E "s/export $var=(.*)/\\1/")
  if [[ -z "$value" ]]; then
    read -rp "Enter value for $var: " value
    echo "export $var=$value" | sudo tee -a "$bashrc" > /dev/null
  fi
  env_vars[$var]=$value
done

# Write config.json with extracted or prompted values
cfgdir="$svchome/.config/swpc-alerts"
sudo -u "$svcuser" mkdir -p "$cfgdir"
sudo tee "$cfgdir/config.json" > /dev/null <<EOF
{
  "twilio_sid": "${env_vars[TWILIO_SID]}",
  "twilio_auth": "${env_vars[TWILIO_TOKEN]}",
  "twilio_from": "${env_vars[TWILIO_FROM]}",
  "twilio_to": "+11234567890",
  "dry_run": true,
  "check_interval_minutes": 15,
  "kp_threshold": 7.0,
  "bz_threshold": -8.0,
  "proton_flux_threshold": 0.1,
  "xray_flux_threshold": 0.0001
}
EOF

sudo chown -R "$svcuser:$svcuser" "$cfgdir"

# Set up systemd --user service unit
svcunit="$svchome/.config/systemd/user/space-alerts.service"
sudo -u "$svcuser" mkdir -p "$(dirname "$svcunit")"
sudo tee "$svcunit" > /dev/null <<EOF
[Unit]
Description=Space Weather Alert Monitor
After=network.target

[Service]
Type=simple
ExecStart=$svchome/space_alerts
Restart=on-failure
Environment=HOME=$svchome

[Install]
WantedBy=default.target
EOF

# Output instructions for completing user service setup
echo ""
echo "✅ Service unit created for $svcuser."
echo "To start it, run the following as that user:"
echo ""
echo "    runuser -l $svcuser -c 'systemctl --user daemon-reexec && systemctl --user daemon-reload && systemctl --user enable --now space-alerts.service'"
echo ""

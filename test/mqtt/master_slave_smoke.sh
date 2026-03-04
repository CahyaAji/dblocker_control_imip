#!/usr/bin/env bash
set -euo pipefail

# Quick master/slave MQTT validation helper
# Usage examples:
#   ./test/mqtt/master_slave_smoke.sh listen
#   ./test/mqtt/master_slave_smoke.sh status
#   ./test/mqtt/master_slave_smoke.sh sleep
#   ./test/mqtt/master_slave_smoke.sh reset-slave
#   ./test/mqtt/master_slave_smoke.sh wake-rst
#   ./test/mqtt/master_slave_smoke.sh mask 0x0000
#   ./test/mqtt/master_slave_smoke.sh run-all

BROKER_HOST="${MQTT_HOST:-localhost}"
BROKER_PORT="${MQTT_PORT:-1883}"
MQTT_USER="${MQTT_USERNAME:-DBL0KER}"
MQTT_PASS="${MQTT_PASSWORD-}"
if [[ -z "$MQTT_PASS" ]]; then
  MQTT_PASS='4;1Yf,)`'
fi
SERIAL="${DBLOCKER_SERIAL:-250006}"

TOPIC_CMD="dbl/${SERIAL}/cmd"
TOPIC_STA="dbl/${SERIAL}/sta"
TOPIC_RPT="dbl/${SERIAL}/rpt"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing command: $1"
    exit 1
  }
}

pub_text() {
  local payload="$1"
  mosquitto_pub -h "$BROKER_HOST" -p "$BROKER_PORT" -u "$MQTT_USER" -P "$MQTT_PASS" \
    -t "$TOPIC_CMD" -m "$payload" -q 1
}

pub_mask_hex16() {
  local mask_in="$1"
  local mask

  if [[ "$mask_in" =~ ^0x[0-9A-Fa-f]{1,4}$ ]]; then
    mask=$((mask_in))
  elif [[ "$mask_in" =~ ^[0-9]{1,5}$ ]]; then
    mask="$mask_in"
  else
    echo "Invalid mask. Use decimal (e.g. 16383) or hex (e.g. 0x3FFF)."
    exit 1
  fi

  if (( mask < 0 || mask > 65535 )); then
    echo "Mask out of range: $mask (must be 0..65535)"
    exit 1
  fi

  local hi lo
  hi=$(( (mask >> 8) & 0xFF ))
  lo=$(( mask & 0xFF ))

  # Publish raw 2-byte payload (big-endian) expected by master firmware.
  python3 - <<PY | mosquitto_pub -h "$BROKER_HOST" -p "$BROKER_PORT" -u "$MQTT_USER" -P "$MQTT_PASS" -t "$TOPIC_CMD" -s -q 1
import sys
mask = int("$mask")
sys.stdout.buffer.write(bytes([(mask >> 8) & 0xFF, mask & 0xFF]))
PY

  echo "Published 2-byte mask: $mask (0x$(printf '%04X' "$mask"))"
}

status_once() {
  mosquitto_sub -h "$BROKER_HOST" -p "$BROKER_PORT" -u "$MQTT_USER" -P "$MQTT_PASS" \
    -t "$TOPIC_STA" -C 1 -v
}

listen_all() {
  echo "Listening topics: $TOPIC_STA and $TOPIC_RPT (Ctrl+C to stop)"
  mosquitto_sub -h "$BROKER_HOST" -p "$BROKER_PORT" -u "$MQTT_USER" -P "$MQTT_PASS" \
    -t "$TOPIC_STA" -t "$TOPIC_RPT" -v
}

run_all() {
  echo "[1/6] Read retained status"
  status_once || true

  echo "[2/6] Send SLEEP"
  pub_text "SLEEP"
  sleep 1
  status_once || true

  echo "[3/6] Send mask=0x0000 (all off command payload)"
  pub_mask_hex16 "0x0000"
  sleep 1

  echo "[4/6] Send RST_SLAVE"
  pub_text "RST_SLAVE"
  sleep 2

  echo "[5/6] Send mask=0x3FFF (all on command payload bits)"
  pub_mask_hex16 "0x3FFF"
  sleep 1

  echo "[6/6] Send WAKE_RST (master reboot path)"
  pub_text "WAKE_RST"
  echo "Waiting 6s for reboot/reconnect..."
  sleep 6
  status_once || true

  echo "Done."
}

main() {
  require_cmd mosquitto_pub
  require_cmd mosquitto_sub
  require_cmd python3

  local action="${1:-}"
  case "$action" in
    listen)
      listen_all
      ;;
    status)
      status_once
      ;;
    sleep)
      pub_text "SLEEP"
      echo "Sent SLEEP"
      ;;
    reset-slave)
      pub_text "RST_SLAVE"
      echo "Sent RST_SLAVE"
      ;;
    wake-rst)
      pub_text "WAKE_RST"
      echo "Sent WAKE_RST"
      ;;
    mask)
      if [[ $# -lt 2 ]]; then
        echo "Usage: $0 mask <hex|decimal>"
        exit 1
      fi
      pub_mask_hex16 "$2"
      ;;
    run-all)
      run_all
      ;;
    *)
      cat <<USAGE
Usage: $0 <action>

Actions:
  listen            Listen to status/report topics
  status            Read one retained status message
  sleep             Publish SLEEP command
  reset-slave       Publish RST_SLAVE command
  wake-rst          Publish WAKE_RST command
  mask <value>      Publish 2-byte command mask (hex or decimal)
  run-all           Run a quick smoke sequence

Environment overrides:
  MQTT_HOST, MQTT_PORT, MQTT_USERNAME, MQTT_PASSWORD, DBLOCKER_SERIAL
USAGE
      exit 1
      ;;
  esac
}

main "$@"

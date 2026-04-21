#!/usr/bin/env node
// DBLOCKER SIMULATOR (Node.js)
// Simulates the STM32 DBlocker MCU over MQTT.
// Subscribes to cmd topic, publishes sensor data (rpt) and status (sta).
//
// Requires: npm install mqtt
// Usage:    node dblocker_simulator.js

const mqtt = require("mqtt");

// ============================================================
// CONFIG — EDIT PER SIMULATED DEVICE
// These are the defaults. All can be overridden via CLI args:
//   node dblocker_simulator.js --serial 250003 --mqtt-host 10.0.0.1 --mqtt-pass secret
// ============================================================
const SERIAL_NUMB = getArg("--serial",    "250002"); // ⚙️ Device serial number (must match DB)

const MQTT_HOST   = getArg("--mqtt-host", "localhost");  // ⚙️ MQTT broker host (Docker exposes to host)
const MQTT_PORT   = parseInt(getArg("--mqtt-port", "1883"), 10); // ⚙️ MQTT broker port
const MQTT_USER   = getArg("--mqtt-user", "DBL0KER");   // ⚙️ MQTT username
const MQTT_PASS   = getArg("--mqtt-pass", "4;1Yf,)`");  // ⚙️ MQTT password (from mosquitto passwordfile)

const PUBLISH_INTERVAL_MS = parseInt(getArg("--interval", "2000"), 10); // ⚙️ Sensor publish interval (ms)
// Pass --quiet to suppress periodic rpt log (useful when running many simulators)
const QUIET = process.argv.includes("--quiet");

// Fake sensor ADC ranges (0-1023 raw, like STM32 10-bit ADC)
const SENSOR_OFF_MIN = 512;            // ⚙️ ADC value when signal is OFF
const SENSOR_OFF_MAX = 580;            // ⚙️ ADC value when signal is OFF
const SENSOR_ON_MIN  = 550;            // ⚙️ ADC value when signal is ON
const SENSOR_ON_MAX  = 620;            // ⚙️ ADC value when signal is ON
const TEMP_RAW       = 530;            // ⚙️ Raw ADC temperature value
// ============================================================

// CLI argument helper
function getArg(name, defaultVal) {
  const args = process.argv.slice(2);
  const idx = args.indexOf(name);
  if (idx !== -1 && args[idx + 1]) return args[idx + 1];
  return defaultVal;
}

// Topics
const TOPIC_CMD = `dbl/${SERIAL_NUMB}/cmd`;
const TOPIC_RPT = `dbl/${SERIAL_NUMB}/rpt`;
const TOPIC_STA = `dbl/${SERIAL_NUMB}/sta`;

// Output state: 16-bit mask matching MCU layout
// Bits 0-6:  master outputs (sectors 0-2 GPS/Ctrl + fan)
// Bits 7-13: slave outputs  (sectors 3-5 GPS/Ctrl + fan)
//
// Per sector bit layout (matches DBlockerConfigToBitmask in backend):
//   bit  0: sector 0 GPS    bit  1: sector 0 Ctrl
//   bit  2: sector 1 GPS    bit  3: sector 1 Ctrl
//   bit  4: sector 2 GPS    bit  5: sector 2 Ctrl
//   bit  6: fan master
//   bit  7: sector 3 GPS    bit  8: sector 3 Ctrl
//   bit  9: sector 4 GPS    bit 10: sector 4 Ctrl
//   bit 11: sector 5 GPS    bit 12: sector 5 Ctrl
//   bit 13: fan slave
let outputMask = 0x0000;

// GPS bit index per sector (0-5)
const GPS_BITS  = [0, 2, 4, 7, 9, 11];
// Ctrl bit index per sector (0-5)
const CTRL_BITS = [1, 3, 5, 8, 10, 12];

let isSleeping = false;

// --- Helpers ---

// Decode outputMask into a human-readable sector summary.
// Example: "S0[GPS+CTRL] S2[CTRL] S4[GPS] ALL-OFF"
function describeMask(mask) {
  if (mask === 0) return "ALL-OFF";
  const parts = [];
  for (let s = 0; s < 6; s++) {
    const gps  = (mask >> GPS_BITS[s])  & 1;
    const ctrl = (mask >> CTRL_BITS[s]) & 1;
    if (gps || ctrl) {
      const signals = [gps ? "GPS" : null, ctrl ? "CTRL" : null].filter(Boolean).join("+");
      parts.push(`S${s}[${signals}]`);
    }
  }
  const fanM = (mask >> 6)  & 1;
  const fanS = (mask >> 13) & 1;
  if (fanM) parts.push("FAN-M");
  if (fanS) parts.push("FAN-S");
  return parts.join(" ") || "ALL-OFF";
}

function randInt(min, max) {
  return min + Math.floor(Math.random() * (max - min + 1));
}

function sensorValue(isOn) {
  return isOn
    ? randInt(SENSOR_ON_MIN, SENSOR_ON_MAX)
    : randInt(SENSOR_OFF_MIN, SENSOR_OFF_MAX);
}

// Build /rpt payload: 18 sensor values + tempRaw | slaveConnected
// Fields 0-17: 6 sectors × 3 sensors [GPS, Ctrl1, Ctrl2]
// Fields 18: temperature raw ADC
function buildRptPayload() {
  const fields = [];
  for (let s = 0; s < 6; s++) {
    const gpsOn  = (outputMask >> GPS_BITS[s])  & 1;
    const ctrlOn = (outputMask >> CTRL_BITS[s]) & 1;
    fields.push(sensorValue(gpsOn));   // GPS
    fields.push(sensorValue(ctrlOn));  // Ctrl1
    fields.push(sensorValue(ctrlOn));  // Ctrl2
  }
  fields.push(TEMP_RAW);
  return fields.join(",") + "|1";
}

function buildStaPayload() {
  return `ON:${outputMask.toString(16).toUpperCase().padStart(4, "0")}`;
}

// Apply a 16-bit mask from a 2-byte MQTT command payload
function applyMask(buf) {
  outputMask = ((buf[0] << 8) | buf[1]) & 0xffff;
  isSleeping = false;
}

// --- MQTT ---

const client = mqtt.connect({
  host:     MQTT_HOST,
  port:     MQTT_PORT,
  username: MQTT_USER,
  password: MQTT_PASS,
  clientId: SERIAL_NUMB,
  will: {
    topic:   TOPIC_STA,
    payload: "OFF",
    retain:  true,
    qos:     1,
  },
  clean: true,
});

client.on("connect", () => {
  console.log(`[SIM:${SERIAL_NUMB}] Connected to MQTT broker ${MQTT_HOST}:${MQTT_PORT}`);
  client.subscribe(TOPIC_CMD, (err) => {
    if (err) console.error(`[SIM:${SERIAL_NUMB}] Subscribe error:`, err);
  });

  // Publish initial status
  client.publish(TOPIC_STA, buildStaPayload(), { retain: true });
  console.log(`[SIM:${SERIAL_NUMB}] Published initial status: ${buildStaPayload()}`);
});

client.on("message", (topic, payload) => {
  if (topic !== TOPIC_CMD) return;

  const len = payload.length;

  if (len === 5 && payload.toString() === "SLEEP") {
    isSleeping = true;
    outputMask = 0x0000;
    client.publish(TOPIC_STA, "SLEEP", { retain: true });
    console.log(`[SIM:${SERIAL_NUMB}] SLEEP command received`);
    return;
  }

  if (len === 4 && payload.toString() === "WAKE") {
    isSleeping = false;
    client.publish(TOPIC_STA, buildStaPayload(), { retain: true });
    console.log(`[SIM:${SERIAL_NUMB}] WAKE command received`);
    return;
  }

  if (len === 8 && payload.toString() === "WAKE_RST") {
    isSleeping = false;
    outputMask = 0x0000;
    client.publish(TOPIC_STA, "OFF", { retain: true });
    console.log(`[SIM:${SERIAL_NUMB}] WAKE_RST — simulating reboot...`);
    setTimeout(() => {
      client.publish(TOPIC_STA, buildStaPayload(), { retain: true });
      console.log(`[SIM:${SERIAL_NUMB}] Back online after reboot`);
    }, 2000);
    return;
  }

  if (len === 9 && payload.toString() === "RST_SLAVE") {
    console.log(`[SIM:${SERIAL_NUMB}] RST_SLAVE — ignored (no slave in simulator)`);
    return;
  }

  if (len === 2) {
    applyMask(payload);
    const sta = buildStaPayload();
    client.publish(TOPIC_STA, sta, { retain: true });
    console.log(`[SIM:${SERIAL_NUMB}] ${describeMask(outputMask)}  (mask 0x${outputMask.toString(16).toUpperCase().padStart(4, "0")})`);
    return;
  }

  console.log(`[SIM:${SERIAL_NUMB}] Unknown command (${len} bytes): ${payload.toString("hex")}`);
});

client.on("error", (err) => {
  console.error(`[SIM:${SERIAL_NUMB}] MQTT error:`, err.message);
});

client.on("offline", () => {
  console.log(`[SIM:${SERIAL_NUMB}] MQTT offline, reconnecting...`);
});

// Periodic sensor publish
setInterval(() => {
  if (!client.connected || isSleeping) return;
  const rpt = buildRptPayload();
  client.publish(TOPIC_RPT, rpt);
  if (!QUIET) console.log(`[SIM:${SERIAL_NUMB}] rpt: ${rpt}`);
}, PUBLISH_INTERVAL_MS);

console.log("=== DBlocker Simulator (Node.js) ===");
console.log(`Serial: ${SERIAL_NUMB} | MQTT: ${MQTT_HOST}:${MQTT_PORT}`);
console.log(`Topics: cmd=${TOPIC_CMD}  rpt=${TOPIC_RPT}  sta=${TOPIC_STA}`);

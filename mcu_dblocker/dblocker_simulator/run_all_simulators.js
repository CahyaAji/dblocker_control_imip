#!/usr/bin/env node
// Run all 10 DBlocker simulators as child processes.
// Each one connects to MQTT with its own serial number.
//
// Usage: node run_all_simulators.js

const { spawn } = require("child_process");

// ============================================================
// SHARED MQTT CONFIG — applied to all simulators
// MQTT runs in Docker, exposed on host port 1883.
// Credentials must match mosquitto/config/passwordfile.
// ============================================================
const MQTT_HOST = "localhost";    // ⚙️ MQTT broker host (Docker exposes to host)
const MQTT_PASS = "4;1Yf,)`";    // ⚙️ MQTT password (from mosquitto passwordfile)
const MQTT_USER = "DBL0KER";     // ⚙️ MQTT username
const MQTT_PORT = "1883";        // ⚙️ MQTT broker port
// ============================================================

// ============================================================
// DEVICE LIST — add/remove/edit entries here
// ============================================================
const devices = [
  { serial: "250001" },
  { serial: "250002" },
  { serial: "250003" },
  { serial: "250004" },
  { serial: "250005" },
  { serial: "250006" },
  { serial: "250007" },
  { serial: "250008" },
  { serial: "250009" },
  { serial: "250010" },
];
// ============================================================

const SIMULATOR_SCRIPT = `${__dirname}/dblocker_simulator.js`;

devices.forEach(({ serial }) => {
  const args = [
    SIMULATOR_SCRIPT,
    "--serial",    serial,
    "--mqtt-host", MQTT_HOST,
    "--mqtt-port", MQTT_PORT,
    "--mqtt-user", MQTT_USER,
    "--mqtt-pass", MQTT_PASS,
    "--quiet",  // suppress per-tick rpt logs; remove to see raw sensor data
  ];

  const proc = spawn("node", args, { stdio: "pipe" });

  proc.stdout.on("data", (data) => {
    process.stdout.write(`[${serial}] ${data}`);
  });

  proc.stderr.on("data", (data) => {
    process.stderr.write(`[${serial}] ERR: ${data}`);
  });

  proc.on("exit", (code) => {
    console.log(`[${serial}] process exited with code ${code}`);
  });

  console.log(`[run_all] Started simulator for serial ${serial} (pid ${proc.pid})`);
});

console.log(`[run_all] ${devices.length} simulators running. Press Ctrl+C to stop all.`);

// Kill all children on Ctrl+C
process.on("SIGINT", () => {
  console.log("\n[run_all] Stopping all simulators...");
  process.exit(0);
});

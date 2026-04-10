#!/usr/bin/env node
// DRONE DETECTOR SIMULATOR (Node.js / Raspberry Pi)
// Simulates the binary TCP protocol of a drone detector device.
// Acts as a TCP server. When the assist service connects,
// it sends periodic heartbeat frames (type 1) and simulated drone
// detection frames (type 56) so you can test the full pipeline.
//
// Usage: node detector_simulator.js [--port 5555] [--mode always|cycle|manual]
//
// Modes:
//   always  - Sends drone detections continuously (default)
//   cycle   - Alternates between idle (no drone) and detection periods
//   manual  - Press Enter to toggle drone on/off

// # Always mode (default)
// node detector_simulator.js
// # Cycle mode: 30s idle, 20s detection, repeat
// node detector_simulator.js --mode cycle
// # Manual mode: press Enter to toggle
// node detector_simulator.js --mode manual
// # Combine with port
// node detector_simulator.js --port 5555 --mode cycle

const net = require("net");
const readline = require("readline");

// ============ CONFIG ============
const TCP_PORT = parseInt(process.env.TCP_PORT || "5555", 10);

// Device info (heartbeat)
const DEVICE_ID = 1001;
const DEVICE_NAME = "SimDetector1";
const DEVICE_TYPE = "SIM";
const DEVICE_LAT = -2.81968;
const DEVICE_LNG = 122.15309;
const DEVICE_ALT = 50;

// Simulated drone parameters
const DRONE_UID = "SIM-DRONE-001";
const DRONE_NAME = "DJI Mavic Sim";
const DRONE_BASE_LAT = -2.81;
const DRONE_BASE_LNG = 122.14;
const DRONE_ALT = 120;
const DRONE_CONFIDENCE = 85;

// Timing
const HEARTBEAT_INTERVAL_MS = 10000; // 10s
const DRONE_INTERVAL_MS = 3000; // 3s

// Simulation mode: "always" | "cycle" | "manual"
const SIM_MODE = process.env.SIM_MODE || "always";

// Cycle mode durations (easy to change)
const IDLE_DURATION_S = 30;       // seconds with no drone (cycle mode)
const DETECTION_DURATION_S = 20;  // seconds with active drone (cycle mode)
// ================================

const DEG_TO_RAD = Math.PI / 180;

let droneAngle = 0.0;

// --- Binary helpers (Little-Endian) ---

function writeUint16LE(buf, offset, val) {
  buf.writeUInt16LE(val & 0xffff, offset);
}

function writeUint32LE(buf, offset, val) {
  buf.writeUInt32LE(val >>> 0, offset);
}

function writeInt32LE(buf, offset, val) {
  buf.writeInt32LE(val, offset);
}

function writeFloat32LE(buf, offset, val) {
  buf.writeFloatLE(val, offset);
}

function writeFloat64LE(buf, offset, val) {
  buf.writeDoubleLE(val, offset);
}

function writeStringPadded(buf, offset, str, padLen) {
  buf.fill(0, offset, offset + padLen);
  const len = Math.min(Buffer.byteLength(str, "utf8"), padLen);
  buf.write(str.substring(0, len), offset, len, "utf8");
}

// --- Frame builder ---

function buildFrame(dataType, dataSection) {
  const dataLen = dataSection.length;
  const frameLen = 29 + dataLen + 5; // header + data + trailer
  const frame = Buffer.alloc(frameLen, 0);

  // Start flag 0xEEEEEEEE
  frame[0] = 0xee;
  frame[1] = 0xee;
  frame[2] = 0xee;
  frame[3] = 0xee;

  // Bytes 6-9: frame length
  writeUint32LE(frame, 6, frameLen);

  // Bytes 19-20: data type
  writeUint16LE(frame, 19, dataType);

  // Data section at offset 29
  dataSection.copy(frame, 29);

  // Checksum placeholder at frameLen - 5
  frame[frameLen - 5] = 0x00;

  // End flag 0xAAAAAAAA
  frame[frameLen - 4] = 0xaa;
  frame[frameLen - 3] = 0xaa;
  frame[frameLen - 2] = 0xaa;
  frame[frameLen - 1] = 0xaa;

  return frame;
}

// --- Heartbeat (type 1) - 74 bytes ---

function buildHeartbeat() {
  const data = Buffer.alloc(74, 0);

  writeInt32LE(data, 0, DEVICE_ID);
  writeStringPadded(data, 4, DEVICE_NAME, 20);
  writeFloat32LE(data, 24, DEVICE_LNG);
  writeFloat32LE(data, 28, DEVICE_LAT);
  writeInt32LE(data, 32, DEVICE_ALT);
  writeUint16LE(data, 36, 1); // OpStatus: Working
  writeFloat32LE(data, 38, 0.0); // Azimuth
  writeStringPadded(data, 42, DEVICE_TYPE, 4);
  data[46] = 0; // CompassStatus: Normal
  data[47] = 0; // GPSStatus: Normal
  data[48] = 1; // RFSwitchStatus: Normal
  data[49] = 0; // ConnectionStatus: Connected
  writeInt32LE(data, 50, 360); // CoverageArea
  writeInt32LE(data, 54, 0); // RecvDeviceID
  writeFloat32LE(data, 58, 25.5); // Temperature
  writeFloat32LE(data, 62, 65.0); // Humidity

  return buildFrame(1, data);
}

// --- Drone target (type 56) ---

function buildDroneTarget() {
  droneAngle += 15.0;
  if (droneAngle >= 360.0) droneAngle -= 360.0;

  const radius = 0.005; // ~500m in degrees
  const droneLat =
    DRONE_BASE_LAT + radius * Math.cos(droneAngle * DEG_TO_RAD);
  const droneLng =
    DRONE_BASE_LNG + radius * Math.sin(droneAngle * DEG_TO_RAD);
  const heading = Math.floor(droneAngle);
  const distance = 800 + Math.floor(200.0 * Math.sin(droneAngle * DEG_TO_RAD));
  const speed = 12.5 + 3.0 * Math.sin(droneAngle * DEG_TO_RAD * 2.0);

  const nameLen = Buffer.byteLength(DRONE_NAME, "utf8");
  const dataLen = 24 + nameLen + 69;
  const data = Buffer.alloc(dataLen, 0);

  writeStringPadded(data, 0, DRONE_UID, 16);
  writeInt32LE(data, 16, 1); // TargetID
  writeUint32LE(data, 20, nameLen);
  data.write(DRONE_NAME, 24, nameLen, "utf8");

  const off = 24 + nameLen;
  writeFloat32LE(data, off, droneLng);
  writeFloat32LE(data, off + 4, droneLat);
  writeInt32LE(data, off + 8, DRONE_ALT);
  writeInt32LE(data, off + 12, DRONE_ALT - 5);
  writeInt32LE(data, off + 16, heading);
  writeInt32LE(data, off + 20, distance);
  writeFloat32LE(data, off + 24, droneLng + 0.002);
  writeFloat32LE(data, off + 28, droneLat + 0.001);
  writeFloat64LE(data, off + 32, 2437000.0); // Frequency
  writeFloat64LE(data, off + 40, 20000.0); // Bandwidth
  writeFloat64LE(data, off + 48, -45.0); // SignalStrength
  data[off + 56] = DRONE_CONFIDENCE;
  writeUint32LE(data, off + 57, Math.floor(Date.now() / 1000)); // Timestamp
  writeFloat64LE(data, off + 61, speed);

  return { frame: buildFrame(56, data), heading, distance, droneLat, droneLng };
}

// --- Simulation state ---

let droneActive = false; // Whether drone detections are being sent

// Cycle mode: manages idle/detection transitions
function startCycleMode() {
  let detecting = false;

  function toggle() {
    detecting = !detecting;
    droneActive = detecting;
    if (detecting) {
      console.log(`[SIM] Cycle: drone DETECTED — sending targets for ${DETECTION_DURATION_S}s`);
      setTimeout(toggle, DETECTION_DURATION_S * 1000);
    } else {
      console.log(`[SIM] Cycle: drone GONE — idle for ${IDLE_DURATION_S}s`);
      setTimeout(toggle, IDLE_DURATION_S * 1000);
    }
  }

  // Start with idle period
  console.log(`[SIM] Cycle mode: idle ${IDLE_DURATION_S}s → detect ${DETECTION_DURATION_S}s → repeat`);
  droneActive = false;
  setTimeout(toggle, IDLE_DURATION_S * 1000);
}

// Manual mode: toggle drone with Enter key
function startManualMode() {
  droneActive = false;
  console.log("[SIM] Manual mode: press Enter to toggle drone on/off");
  console.log("[SIM] Drone is OFF");

  const rl = readline.createInterface({ input: process.stdin });
  rl.on("line", () => {
    droneActive = !droneActive;
    console.log(`[SIM] Drone is now ${droneActive ? "ON — sending detections" : "OFF — idle"}`);
  });
}

// --- TCP Server ---

const server = net.createServer((socket) => {
  const remote = `${socket.remoteAddress}:${socket.remotePort}`;
  console.log(`[SIM] Client connected from ${remote}`);

  const heartbeatTimer = setInterval(() => {
    if (socket.destroyed) return;
    socket.write(buildHeartbeat());
    console.log("[SIM] Sent heartbeat");
  }, HEARTBEAT_INTERVAL_MS);

  const droneTimer = setInterval(() => {
    if (socket.destroyed) return;
    if (!droneActive) return; // Skip when no drone
    const { frame, heading, distance, droneLat, droneLng } =
      buildDroneTarget();
    socket.write(frame);
    console.log(
      `[SIM] Sent drone target: heading=${heading}° dist=${distance}m lat=${droneLat.toFixed(5)} lng=${droneLng.toFixed(5)}`
    );
  }, DRONE_INTERVAL_MS);

  // Send initial heartbeat immediately
  socket.write(buildHeartbeat());
  console.log("[SIM] Sent heartbeat");

  socket.on("close", () => {
    console.log(`[SIM] Client disconnected: ${remote}`);
    clearInterval(heartbeatTimer);
    clearInterval(droneTimer);
  });

  socket.on("error", (err) => {
    console.log(`[SIM] Socket error (${remote}): ${err.message}`);
    clearInterval(heartbeatTimer);
    clearInterval(droneTimer);
  });

  // Drain incoming data (not used)
  socket.on("data", () => {});
});

// Parse CLI args
const args = process.argv.slice(2);
let port = TCP_PORT;
const portIdx = args.indexOf("--port");
if (portIdx !== -1 && args[portIdx + 1]) {
  port = parseInt(args[portIdx + 1], 10);
}

let mode = SIM_MODE;
const modeIdx = args.indexOf("--mode");
if (modeIdx !== -1 && args[modeIdx + 1]) {
  mode = args[modeIdx + 1];
}

// Initialize simulation mode
switch (mode) {
  case "cycle":
    startCycleMode();
    break;
  case "manual":
    startManualMode();
    break;
  case "always":
  default:
    droneActive = true;
    console.log("[SIM] Always mode: drone detections sent continuously");
    break;
}

server.listen(port, () => {
  console.log("=== Drone Detector Simulator (Node.js) ===");
  console.log(`TCP server listening on port ${port}`);
  console.log(`Mode: ${mode} | Heartbeat every ${HEARTBEAT_INTERVAL_MS / 1000}s | Drone target every ${DRONE_INTERVAL_MS / 1000}s`);
});

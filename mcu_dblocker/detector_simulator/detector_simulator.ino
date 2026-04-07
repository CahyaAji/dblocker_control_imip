// DRONE DETECTOR SIMULATOR (ESP32)
// Simulates the binary TCP protocol of a drone detector device.
// Acts as a TCP server on port 5555. When the assist service connects,
// it sends periodic heartbeat frames (type 1) and simulated drone
// detection frames (type 56) so you can test the full pipeline.
//
// WiFi credentials and drone simulation parameters are configurable below.

#include <WiFi.h>

// ============ CONFIG ============
const char* WIFI_SSID     = "YOUR_SSID";
const char* WIFI_PASSWORD = "YOUR_PASSWORD";
const uint16_t TCP_PORT   = 5555;

// Device info (heartbeat)
const int32_t  DEVICE_ID   = 1001;
const char     DEVICE_NAME[] = "SimDetector1";
const char     DEVICE_TYPE[] = "SIM";
const float    DEVICE_LAT  = -2.81968f;
const float    DEVICE_LNG  = 122.15309f;
const int32_t  DEVICE_ALT  = 50;

// Simulated drone parameters
const char     DRONE_UID[]    = "SIM-DRONE-001";
const char     DRONE_NAME[]   = "DJI Mavic Sim";
const float    DRONE_BASE_LAT = -2.810f;
const float    DRONE_BASE_LNG = 122.140f;
const int32_t  DRONE_ALT      = 120;
const uint8_t  DRONE_CONFIDENCE = 85;

// Timing
const unsigned long HEARTBEAT_INTERVAL_MS = 10000;  // 10s
const unsigned long DRONE_INTERVAL_MS     = 3000;   // 3s
// ================================

WiFiServer server(TCP_PORT);
WiFiClient client;

unsigned long lastHeartbeat = 0;
unsigned long lastDrone     = 0;
float droneAngle = 0.0f;  // Simulates drone circling

// Frame header: 29 bytes before data section
// Frame trailer: 1 byte (checksum placeholder) + 4 bytes end flag = 5 bytes
// Total frame = 29 + dataLen + 5 = 34 + dataLen

void writeUint16LE(uint8_t* buf, uint16_t val) {
  buf[0] = val & 0xFF;
  buf[1] = (val >> 8) & 0xFF;
}

void writeUint32LE(uint8_t* buf, uint32_t val) {
  buf[0] = val & 0xFF;
  buf[1] = (val >> 8) & 0xFF;
  buf[2] = (val >> 16) & 0xFF;
  buf[3] = (val >> 24) & 0xFF;
}

void writeInt32LE(uint8_t* buf, int32_t val) {
  writeUint32LE(buf, (uint32_t)val);
}

void writeFloat32LE(uint8_t* buf, float val) {
  uint32_t bits;
  memcpy(&bits, &val, 4);
  writeUint32LE(buf, bits);
}

void writeFloat64LE(uint8_t* buf, double val) {
  uint64_t bits;
  memcpy(&bits, &val, 8);
  for (int i = 0; i < 8; i++) {
    buf[i] = (bits >> (i * 8)) & 0xFF;
  }
}

void writeStringPadded(uint8_t* buf, const char* str, int padLen) {
  memset(buf, 0, padLen);
  int len = strlen(str);
  if (len > padLen) len = padLen;
  memcpy(buf, str, len);
}

// Build and send a complete frame over TCP
// dataType: 1 = heartbeat, 56 = drone target
void sendFrame(WiFiClient& c, uint16_t dataType, uint8_t* dataSection, int dataLen) {
  int frameLen = 29 + dataLen + 5;  // header + data + trailer
  uint8_t* frame = (uint8_t*)malloc(frameLen);
  if (!frame) return;

  memset(frame, 0, frameLen);

  // Start flag (0xEEEEEEEE)
  frame[0] = 0xEE; frame[1] = 0xEE; frame[2] = 0xEE; frame[3] = 0xEE;

  // Bytes 4-5: reserved (zeroed)
  // Bytes 6-9: frame length
  writeUint32LE(&frame[6], (uint32_t)frameLen);

  // Bytes 10-18: header fields (zeroed for simulator)
  // Bytes 19-20: data type
  writeUint16LE(&frame[19], dataType);

  // Bytes 21-28: more header (zeroed)

  // Data section at offset 29
  memcpy(&frame[29], dataSection, dataLen);

  // Trailer: 1 byte checksum (0x00 placeholder) at frameLen-5
  frame[frameLen - 5] = 0x00;

  // End flag (0xAAAAAAAA) at last 4 bytes
  frame[frameLen - 4] = 0xAA;
  frame[frameLen - 3] = 0xAA;
  frame[frameLen - 2] = 0xAA;
  frame[frameLen - 1] = 0xAA;

  c.write(frame, frameLen);
  free(frame);
}

// Build heartbeat data (type 1) - 74 bytes
void sendHeartbeat(WiFiClient& c) {
  uint8_t data[74];
  memset(data, 0, 74);

  writeInt32LE(&data[0], DEVICE_ID);                    // DeviceID
  writeStringPadded(&data[4], DEVICE_NAME, 20);         // DeviceName (20 bytes)
  writeFloat32LE(&data[24], DEVICE_LNG);                // Longitude
  writeFloat32LE(&data[28], DEVICE_LAT);                // Latitude
  writeInt32LE(&data[32], DEVICE_ALT);                  // Altitude
  writeUint16LE(&data[36], 1);                          // OpStatus: 1=Working
  writeFloat32LE(&data[38], 0.0f);                      // Azimuth
  writeStringPadded(&data[42], DEVICE_TYPE, 4);         // DeviceType (4 bytes)
  data[46] = 0;   // CompassStatus: 0=Normal
  data[47] = 0;   // GPSStatus: 0=Normal
  data[48] = 1;   // RFSwitchStatus: 1=Normal
  data[49] = 0;   // ConnectionStatus: 0=Connected
  writeInt32LE(&data[50], 360);                         // CoverageArea
  writeInt32LE(&data[54], 0);                           // RecvDeviceID
  writeFloat32LE(&data[58], 25.5f);                     // Temperature
  writeFloat32LE(&data[62], 65.0f);                     // Humidity
  // bytes 66-73 reserved/padding

  sendFrame(c, 1, data, 74);
  Serial.println("[SIM] Sent heartbeat");
}

// Build drone target data (type 56) - variable length
void sendDroneTarget(WiFiClient& c) {
  // Simulate drone circling around a point
  droneAngle += 15.0f;
  if (droneAngle >= 360.0f) droneAngle -= 360.0f;

  float radius = 0.005f;  // ~500m in degrees
  float droneLat = DRONE_BASE_LAT + radius * cos(droneAngle * DEG_TO_RAD);
  float droneLng = DRONE_BASE_LNG + radius * sin(droneAngle * DEG_TO_RAD);
  int32_t heading = (int32_t)droneAngle;
  int32_t distance = 800 + (int32_t)(200.0f * sin(droneAngle * DEG_TO_RAD));
  float speed = 12.5f + 3.0f * sin(droneAngle * DEG_TO_RAD * 2.0f);

  int nameLen = strlen(DRONE_NAME);

  // Data layout:
  // [0:16]   UniqueID (16 bytes, null-padded)
  // [16:20]  TargetID (int32)
  // [20:24]  NameLength (uint32)
  // [24:24+nameLen] TargetName
  // Then 69 bytes of target data

  int dataLen = 24 + nameLen + 69;
  uint8_t* data = (uint8_t*)malloc(dataLen);
  if (!data) return;
  memset(data, 0, dataLen);

  writeStringPadded(&data[0], DRONE_UID, 16);           // UniqueID
  writeInt32LE(&data[16], 1);                            // TargetID
  writeUint32LE(&data[20], (uint32_t)nameLen);           // NameLength
  memcpy(&data[24], DRONE_NAME, nameLen);                // TargetName

  int off = 24 + nameLen;
  writeFloat32LE(&data[off],      droneLng);             // DroneLongitude
  writeFloat32LE(&data[off + 4],  droneLat);             // DroneLatitude
  writeInt32LE(&data[off + 8],    DRONE_ALT);            // DroneAltitude
  writeInt32LE(&data[off + 12],   DRONE_ALT - 5);        // BaroAltitude
  writeInt32LE(&data[off + 16],   heading);              // DirectionAngle
  writeInt32LE(&data[off + 20],   distance);             // Distance
  writeFloat32LE(&data[off + 24], droneLng + 0.002f);   // RemoteLongitude
  writeFloat32LE(&data[off + 28], droneLat + 0.001f);   // RemoteLatitude
  writeFloat64LE(&data[off + 32], 2437000.0);            // Frequency (kHz)
  writeFloat64LE(&data[off + 40], 20000.0);              // Bandwidth (kHz)
  writeFloat64LE(&data[off + 48], -45.0);                // SignalStrength (dB)
  data[off + 56] = DRONE_CONFIDENCE;                     // Confidence
  writeUint32LE(&data[off + 57],  (uint32_t)(millis() / 1000)); // Timestamp
  writeFloat64LE(&data[off + 61], (double)speed);        // FlightSpeed

  sendFrame(c, 56, data, dataLen);
  Serial.printf("[SIM] Sent drone target: heading=%d° dist=%dm lat=%.5f lng=%.5f\n",
                heading, distance, droneLat, droneLng);
  free(data);
}

void setup() {
  Serial.begin(115200);
  delay(1000);
  Serial.println("\n=== Drone Detector Simulator (ESP32) ===");

  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.print("Connecting to WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.printf("\nConnected! IP: %s\n", WiFi.localIP().toString().c_str());

  server.begin();
  Serial.printf("TCP server listening on port %d\n", TCP_PORT);
}

void loop() {
  // Accept new client
  if (!client || !client.connected()) {
    WiFiClient newClient = server.available();
    if (newClient) {
      client = newClient;
      Serial.printf("[SIM] Client connected from %s\n", client.remoteIP().toString().c_str());
      lastHeartbeat = 0;
      lastDrone = 0;
    }
    return;
  }

  unsigned long now = millis();

  // Send heartbeat periodically
  if (now - lastHeartbeat >= HEARTBEAT_INTERVAL_MS) {
    sendHeartbeat(client);
    lastHeartbeat = now;
  }

  // Send drone target periodically
  if (now - lastDrone >= DRONE_INTERVAL_MS) {
    sendDroneTarget(client);
    lastDrone = now;
  }

  // Drain any incoming data (we don't use it)
  while (client.available()) {
    client.read();
  }

  delay(10);
}

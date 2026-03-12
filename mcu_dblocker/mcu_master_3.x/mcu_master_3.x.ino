// MASTER (STM32F411CEU6) v3.1
#include <SPI.h>
#include <Ethernet.h>
#include <PubSubClient.h>
#include <ctype.h>
#include <IWatchdog.h>

// Config ========================
const bool USE_RS485 = false;
const unsigned long SAFETY_SHUTDOWN_TIMEOUT = 20000;
const unsigned long REBOOT_TIMEOUT = 300000;

#define LED_PIN PC13
#define CMD_PIN PA11

// Ethernet
#define W5500_SCK PB3
#define W5500_MISO PB4
#define W5500_MOSI PB5
#define W5500_CS PA15
#define W5500_RST PC15

// Analog Temperature Sensor
#define TEMP_SENSOR_PIN PB1

HardwareSerial SlaveSerial(PA10, PA9);

uint32_t outPins[7] = { PB10, PB12, PA12, PB6, PB7, PB8, PB9 };
uint32_t hallSensorPins[9] = { PB0, PA7, PA6, PA5, PA4, PA3, PA2, PA1, PA0 };

// Config ========================
// EDIT PER CONTROLLER ========================
const char controller_id[] = "250002";
byte mac[] = { 0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0x02 };
IPAddress ip(10, 88, 81, 3);

// --- SECURITY SETTINGS ---
const char TCP_SECRET[] = "p!ml_3rUc35";
// ===========================================

IPAddress gateway(10, 88, 81, 1);
IPAddress subnet(255, 255, 255, 0);
IPAddress myDns(8, 8, 8, 8);

IPAddress mqtt_broker(10, 88, 81, 16);
const char mqtt_user[] = "DBL0KER";
const char mqtt_pass[] = "4;1Yf,)`";

char serial_numb[10];
char topic_sub[64];
char topic_pub[64];
char topic_sta[64];

int allHallSensors[18];
bool lastSlaveState[7] = { 0 };
bool lastMasterState[7] = { 0 };

unsigned long lastMqttRetry = 0;
unsigned long lastPublish = 0;
unsigned long lastSlaveMessage = 0;
unsigned long lastHeartbeat = 0;
unsigned long lastConnectionTime = 0;
unsigned long lastLinkDownTime = 0;

// Reconnect logic
int mqttReconnectFailures = 0;
const int MAX_MQTT_RECONNECT_FAILURES = 4;
const unsigned long MQTT_RECONNECT_INTERVAL = 3000;

bool slaveConnected = false;
bool isSystemSleeping = false;
bool safetyShutdownActive = false;
bool wasLinkUp = true;
bool wasMqttConnected = false; 

// Serial receive state
char rxBuf[128];
int rxIdx = 0;

EthernetClient ethClient;
PubSubClient mqttClient(ethClient);
EthernetServer tcpServer(8080);

void generateIds() {
  snprintf(serial_numb, sizeof(serial_numb), "%s", controller_id);
  snprintf(topic_sub, sizeof(topic_sub), "dbl/%s/cmd", serial_numb);
  snprintf(topic_pub, sizeof(topic_pub), "dbl/%s/rpt", serial_numb);
  snprintf(topic_sta, sizeof(topic_sta), "dbl/%s/sta", serial_numb);
}

bool isControllerIdValid() {
  return strlen(controller_id) > 0 && strlen(controller_id) < sizeof(serial_numb);
}

uint8_t crc8(const char* data) {
  uint8_t crc = 0;
  while (*data) { crc ^= (uint8_t)(*data++); }
  return crc;
}

void flushSerialBuffer() {
  while (SlaveSerial.available()) {
    SlaveSerial.read();
  }
  rxIdx = 0;
}

void sendToSlave(const char* data) {
  if (USE_RS485) {
    digitalWrite(CMD_PIN, HIGH);
    delayMicroseconds(50);
  }
  SlaveSerial.print(data);
  SlaveSerial.flush();
  if (USE_RS485) {
    delay(2);
    digitalWrite(CMD_PIN, LOW);
  }
}

void sendCommandWithCrc(const char* payload) {
  uint8_t crc = crc8(payload);
  char fullPacket[80];
  snprintf(fullPacket, sizeof(fullPacket), "$%s|%s%X\r\n",
           payload, (crc < 0x10 ? "0" : ""), crc);
  sendToSlave(fullPacket);
}

void syncSlave() {
  char payload[64];
  if (isSystemSleeping || safetyShutdownActive) {
    snprintf(payload, sizeof(payload), "SLEEP");
  } else {
    snprintf(payload, sizeof(payload), "SET:%d,%d,%d,%d,%d,%d,%d",
             lastSlaveState[0], lastSlaveState[1], lastSlaveState[2],
             lastSlaveState[3], lastSlaveState[4], lastSlaveState[5], lastSlaveState[6]);
  }
  sendCommandWithCrc(payload);
}

bool validateAndExtractPayload(char* buf, char* payloadOut, size_t maxLen) {
  char* pipePtr = strchr(buf, '|');
  if (!pipePtr) return false;

  *pipePtr = 0;
  char* crcHex = pipePtr + 1;

  if (strlen(crcHex) != 2 || !isxdigit(crcHex[0]) || !isxdigit(crcHex[1])) return false;

  uint8_t receivedCrc = (uint8_t)strtol(crcHex, NULL, 16);
  if (crc8(buf) != receivedCrc) return false;

  strncpy(payloadOut, buf, maxLen - 1);
  payloadOut[maxLen - 1] = '\0';
  return true;
}

void handleSlaveData(char* buf) {
  char payload[64];
  if (!validateAndExtractPayload(buf, payload, sizeof(payload))) {
    return;
  }

  lastSlaveMessage = millis();

  if (!slaveConnected) {
    slaveConnected = true;
  }

  if (strcmp(payload, "REQ:SYNC") == 0) {
    syncSlave();
    return;
  }

  if (strcmp(payload, "STA:SLEEP") == 0) {
    return;
  }

  if (strncmp(payload, "CUR:", 4) == 0) {
    if (isSystemSleeping) return;

    char* ptr = payload + 4;
    int valuesRead = 0;
    for (int i = 9; i < 18 && valuesRead < 9; i++) {
      if (ptr && *ptr) {
        int val = atoi(ptr);
        if (val < 0) val = 0;
        if (val > 1023) val = 1023;
        allHallSensors[i] = val;
        valuesRead++;
        ptr = strchr(ptr, ',');
        if (ptr) ptr++;
      } else {
        allHallSensors[i] = 0;
      }
    }

    if (valuesRead < 9) {
      for (int i = 9 + valuesRead; i < 18; i++) {
        allHallSensors[i] = 0;
      }
    }
  }
}

void processSlaveSerial() {
  while (SlaveSerial.available()) {
    char c = SlaveSerial.read();
    if (c == '$') {
      rxIdx = 0;
    } else if (c == '\n' || c == '\r') {
      if (rxIdx > 0 && rxIdx < 128) {
        rxBuf[rxIdx] = 0;
        handleSlaveData(rxBuf);
      }
      rxIdx = 0;
    } else if (rxIdx < 127) {
      rxBuf[rxIdx++] = c;
    } else {
      rxIdx = 0;
    }
  }
}

void publishData() {
  if (isSystemSleeping || safetyShutdownActive) {
    return;
  }
  
  for (int i = 0; i < 9; i++) allHallSensors[i] = analogRead(hallSensorPins[i]);
  
  int tempRaw = analogRead(TEMP_SENSOR_PIN);

  if (millis() - lastSlaveMessage > 10000) {
    if (slaveConnected) {
      slaveConnected = false;
      flushSerialBuffer();
    }
    for (int i = 9; i < 18; i++) allHallSensors[i] = 0;
  }

  static char msg[300];
  int offset = 0;

  for (int i = 0; i < 18; i++) {
    int written = snprintf(msg + offset, sizeof(msg) - offset, "%d,", allHallSensors[i]);
    if (written < 0 || offset + written >= sizeof(msg)) {
      msg[offset] = '\0';
      break;
    }
    offset += written;
  }

  int tail = snprintf(msg + offset, sizeof(msg) - offset, "%d|%d", tempRaw, slaveConnected ? 1 : 0);
  if (tail <= 0) msg[offset] = '\0';

  mqttClient.publish(topic_pub, msg);
  digitalWrite(LED_PIN, !digitalRead(LED_PIN));
}

void goToSleep() {
  isSystemSleeping = true;
  for (int i = 0; i < 7; i++) digitalWrite(outPins[i], LOW);
  syncSlave();
  mqttClient.publish(topic_sta, "SLEEP", true);
}

void safetyShutdown() {
  if (safetyShutdownActive) return;
  safetyShutdownActive = true;
  isSystemSleeping = true;
  for (int i = 0; i < 7; i++) digitalWrite(outPins[i], LOW);
  sendCommandWithCrc("SLEEP");
}

void resetW5500() {
  digitalWrite(W5500_RST, LOW);
  delay(100);
  digitalWrite(W5500_RST, HIGH);
  delay(300);

  SPI.setMOSI(W5500_MOSI);
  SPI.setMISO(W5500_MISO);
  SPI.setSCLK(W5500_SCK);
  SPI.begin();
  Ethernet.init(W5500_CS);
  Ethernet.begin(mac, ip, myDns, gateway, subnet);
  ethClient.stop();
  tcpServer.begin();

  mqttClient.setBufferSize(512);
  mqttClient.setServer(mqtt_broker, 1883);
  mqttClient.setCallback(mqttCallback);
}

void publishPinStateToMQTT() {
  if (!mqttClient.connected()) return;

  uint16_t currentMask = 0;
  for (int i = 0; i < 7; i++) {
    if (lastMasterState[i]) currentMask |= (1 << i);
    if (lastSlaveState[i]) currentMask |= (1 << (i + 7));
  }

  char msg[16];
  snprintf(msg, sizeof(msg), "ON:%04X", currentMask);
  mqttClient.publish(topic_sta, msg, true);
}

void mqttCallback(char* topic, byte* payload, unsigned int length) {
  if (length == 5 && memcmp(payload, "SLEEP", 5) == 0) {
    goToSleep();
    return;
  }
  if (length == 4 && memcmp(payload, "WAKE", 4) == 0) {
    isSystemSleeping = false;
    safetyShutdownActive = false;
    for (int i = 0; i < 7; i++) digitalWrite(outPins[i], lastMasterState[i] ? HIGH : LOW);
    syncSlave();
    publishPinStateToMQTT();
    return;
  }
  if (length == 8 && memcmp(payload, "WAKE_RST", 8) == 0) {
    mqttClient.publish(topic_sta, "OFF", true);
    sendCommandWithCrc("RESET");
    delay(500);
    NVIC_SystemReset();
    return;
  }
  if (length == 9 && memcmp(payload, "RST_SLAVE", 9) == 0) {
    sendCommandWithCrc("RESET");
    return;
  }
  if (!isSystemSleeping && length == 2) {
    uint16_t mask = ((uint16_t)payload[0] << 8) | payload[1];
    for (int i = 0; i < 7; i++) {
      bool state = (mask & (1 << i)) ? HIGH : LOW;
      digitalWrite(outPins[i], state);
      lastMasterState[i] = state;
    }
    for (int i = 0; i < 7; i++) lastSlaveState[i] = (mask & (1 << (i + 7)));
    syncSlave();
    safetyShutdownActive = false;
    publishPinStateToMQTT();
  }
}

void setup() {
  IWatchdog.begin(20000000); 
  analogReadResolution(10);

  SlaveSerial.begin(9600);
  
  pinMode(CMD_PIN, OUTPUT);
  digitalWrite(CMD_PIN, LOW);
  
  pinMode(W5500_RST, OUTPUT);
  digitalWrite(W5500_RST, LOW);
  delay(100);
  digitalWrite(W5500_RST, HIGH);
  delay(300);
  
  pinMode(LED_PIN, OUTPUT);

  for (int i = 0; i < 7; i++) {
    pinMode(outPins[i], OUTPUT);
    digitalWrite(outPins[i], LOW);
  }

  generateIds();
  if (!isControllerIdValid()) {
    while (true) {
      digitalWrite(LED_PIN, !digitalRead(LED_PIN));
      delay(100);
    }
  }
  
  SPI.setMOSI(W5500_MOSI);
  SPI.setMISO(W5500_MISO);
  SPI.setSCLK(W5500_SCK);
  SPI.begin();
  Ethernet.init(W5500_CS);
  Ethernet.begin(mac, ip, myDns, gateway, subnet);

  if (Ethernet.hardwareStatus() == EthernetNoHardware) {
    while (true) {
      digitalWrite(LED_PIN, !digitalRead(LED_PIN));
      delay(50);
    }
  }

  mqttClient.setBufferSize(512);
  mqttClient.setServer(mqtt_broker, 1883);
  mqttClient.setCallback(mqttCallback);
  tcpServer.begin();
  syncSlave();
  lastConnectionTime = millis();
  mqttReconnectFailures = 0;
  
  IWatchdog.reload(); 
}

void loop() {
  IWatchdog.reload(); 
  
  unsigned long now = millis();

  if (now - lastHeartbeat > 2000) {
    lastHeartbeat = now;
    syncSlave();
  }

  bool linkUp = (Ethernet.linkStatus() == LinkON);
  if (!linkUp && wasLinkUp) {
    lastLinkDownTime = now;
  }
  wasLinkUp = linkUp;

  if (!linkUp && !safetyShutdownActive) {
    if (now - lastLinkDownTime > SAFETY_SHUTDOWN_TIMEOUT) {
      safetyShutdown();
    }
  }

  bool isMqttConnected = mqttClient.connected();

  if (!isMqttConnected) {
    if (wasMqttConnected) {
      ethClient.stop(); 
      wasMqttConnected = false;
    }

    if (linkUp) {
      if (now - lastMqttRetry > MQTT_RECONNECT_INTERVAL) {
        lastMqttRetry = now;
        ethClient.stop(); 
        
        if (mqttClient.connect(serial_numb, mqtt_user, mqtt_pass, topic_sta, 1, true, "OFF")) {
          wasMqttConnected = true; 
          mqttClient.subscribe(topic_sub);
          
          if (safetyShutdownActive) {
            safetyShutdownActive = false;
            isSystemSleeping = false;
            for (int i = 0; i < 7; i++) digitalWrite(outPins[i], lastMasterState[i] ? HIGH : LOW);
            syncSlave();
          } else {
            syncSlave();
          }
          if (isSystemSleeping) {
            mqttClient.publish(topic_sta, "SLEEP", true);
          } else {
            publishPinStateToMQTT();
          }
          lastConnectionTime = now;
          mqttReconnectFailures = 0;
        } else {
          ethClient.stop(); 
          mqttReconnectFailures++;
          
          if (mqttReconnectFailures >= MAX_MQTT_RECONNECT_FAILURES) {
            resetW5500();
            mqttReconnectFailures = 0;
            lastConnectionTime = now;
            lastMqttRetry = now;
          }
        }
      }
    } else {
      lastMqttRetry = now;
    }

    if (now - lastConnectionTime > SAFETY_SHUTDOWN_TIMEOUT && !safetyShutdownActive) {
      safetyShutdown();
    }

    if (now - lastConnectionTime > REBOOT_TIMEOUT) {
      resetW5500();
      lastConnectionTime = now;
      lastMqttRetry = now;
      mqttReconnectFailures = 0;
    }
  } else {
    wasMqttConnected = true; 
    mqttClient.loop();
    lastConnectionTime = now;
    mqttReconnectFailures = 0;
  }

  EthernetClient tcpClient = tcpServer.available();
  if (tcpClient) {
    char tcpBuf[64];
    int tcpIdx = 0;
    unsigned long tcpStart = millis();

    while (tcpClient.connected()) {
      IWatchdog.reload();
      if (millis() - tcpStart > 2000) {
        tcpClient.println("ERR: TIMEOUT");
        break;
      }
      if (tcpClient.available()) {
        char c = tcpClient.read();

        if (c == '\n' || c == '\r') {
          if (tcpIdx > 0) {
            tcpBuf[tcpIdx] = '\0';
            int secretLen = strlen(TCP_SECRET);
            if (strncmp(tcpBuf, TCP_SECRET, secretLen) == 0 && tcpBuf[secretLen] == ':') {
              char* cmd = tcpBuf + secretLen + 1;

              if (strncmp(cmd, "MASK:", 5) == 0) {
                uint16_t mask = (uint16_t)strtol(cmd + 5, NULL, 16);
                for (int i = 0; i < 7; i++) {
                  bool state = (mask & (1 << i)) ? HIGH : LOW;
                  digitalWrite(outPins[i], state);
                  lastMasterState[i] = state;
                }
                for (int i = 0; i < 7; i++) {
                  lastSlaveState[i] = (mask & (1 << (i + 7))) ? 1 : 0;
                }
                syncSlave();
                publishPinStateToMQTT();
                tcpClient.print("ACK: MASK ");
                tcpClient.println(mask, HEX);
              } else {
                tcpClient.println("ERR: UNKNOWN COMMAND");
              }
            } else {
              tcpClient.println("ERR: UNAUTHORIZED");
            }
            break;
          }
        } else if (tcpIdx < sizeof(tcpBuf) - 1) {
          tcpBuf[tcpIdx++] = c;
        }
      }
    }
    delay(1);
    tcpClient.stop();
  }

  if (now - lastPublish > 2000) {
    lastPublish = now;
    if (mqttClient.connected()) {
      publishData();
    }
  }

  processSlaveSerial(); 
}
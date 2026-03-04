// MASTER (STM32F411CEU6)
#include <SPI.h>
#include <Ethernet.h>
#include <PubSubClient.h>
#include <max6675.h>
#include <IWatchdog.h> 

// Config ========================
const bool USE_RS485 = false; 
const unsigned long SAFETY_SHUTDOWN_TIMEOUT = 30000;  // Turn off outputs after 30s
const unsigned long REBOOT_TIMEOUT = 300000;          // Reboot after 5 minutes

#define LED_PIN PC13
#define CMD_PIN PA11 

// Ethernet
#define W5500_SCK PB3
#define W5500_MISO PB4
#define W5500_MOSI PB5
#define W5500_CS PA15
#define W5500_RST PC15

// Sensors
#define MAX_SCK PB13
#define MAX_MISO PB14
#define MAX_CS_1 PA8  
#define MAX_CS_2 PB1

HardwareSerial SlaveSerial(PA10, PA9); 

uint32_t outPins[7] = { PB8, PB7, PB6, PA12, PB12, PB10, PB9 };
uint32_t hallSensorPins[9] = { PA0, PA1, PA2, PA3, PA4, PA5, PA6, PA7, PB0 };

MAX6675 temp1(MAX_SCK, MAX_CS_1, MAX_MISO);
MAX6675 temp2(MAX_SCK, MAX_CS_2, MAX_MISO);

// Config ========================
// EDIT PER CONTROLLER ========================
// Keep these values unique per device when flashing.
const char controller_id[] = "250006";
byte mac[] = { 0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0x06 };
IPAddress ip(10, 88, 81, 7);
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

unsigned long lastMqttRetry = 0;
unsigned long lastPublish = 0;
unsigned long lastSlaveMessage = 0;
unsigned long lastHeartbeat = 0; 
unsigned long lastConnectionTime = 0;

bool slaveConnected = false;
bool isSystemSleeping = false;
bool safetyShutdownActive = false; 

EthernetClient ethClient;
PubSubClient mqttClient(ethClient);

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

void sendToSlave(const char* data) {
  if (USE_RS485) {
      digitalWrite(CMD_PIN, HIGH); 
      delayMicroseconds(50); 
  }
  SlaveSerial.print(data);
  SlaveSerial.flush();
  if (USE_RS485) {
      delayMicroseconds(500); 
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
  if (isSystemSleeping) {
     snprintf(payload, sizeof(payload), "SLEEP");
  } else {
     snprintf(payload, sizeof(payload), "SET:%d,%d,%d,%d,%d,%d,%d",
           lastSlaveState[0], lastSlaveState[1], lastSlaveState[2],
           lastSlaveState[3], lastSlaveState[4], lastSlaveState[5], lastSlaveState[6]);
  }
  sendCommandWithCrc(payload);
}

void handleSlaveData(char* rxBuf) {
  lastSlaveMessage = millis();
  slaveConnected = true;

  if (strstr(rxBuf, "REQ:SYNC")) {
    syncSlave(); 
    return;
  } 
  
  if (strncmp(rxBuf, "CUR:", 4) == 0) {
    if (isSystemSleeping) {
        syncSlave();
        return;
    }

    char* ptr = rxBuf + 4;
    for (int i = 9; i < 18; i++) {
      if (ptr && *ptr) {
        int val = atoi(ptr);
        allHallSensors[i] = val;
        ptr = strchr(ptr, ',');
        if (ptr) ptr++;
      } else {
        allHallSensors[i] = 0;
      }
    }
  }
}

void publishData() {
  if (isSystemSleeping) {
      mqttClient.publish(topic_sta, "SLEEP", true);
      return; 
  }
  
  for (int i = 0; i < 9; i++) allHallSensors[i] = analogRead(hallSensorPins[i]);
  IWatchdog.reload();
  float t1 = temp1.readCelsius(); delay(5); 
  IWatchdog.reload();
  float t2 = temp2.readCelsius();
  IWatchdog.reload();
  int t1i = isnan(t1) ? -9900 : (int)(t1 * 100);
  int t2i = isnan(t2) ? -9900 : (int)(t2 * 100);

  if (millis() - lastSlaveMessage > 10000) {
    slaveConnected = false;
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

  int tail = snprintf(msg + offset, sizeof(msg) - offset, "%d,%d|%d", t1i, t2i, slaveConnected ? 1 : 0);
  if (tail <= 0) msg[offset] = '\0';

  if (!mqttClient.publish(topic_pub, msg)) {
  }
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
  for (int i = 0; i < 7; i++) digitalWrite(outPins[i], LOW);
  sendCommandWithCrc("SLEEP");
}

void mqttCallback(char* topic, byte* payload, unsigned int length) {
  if (length == 5 && memcmp(payload, "SLEEP", 5) == 0) {
    goToSleep();
    return;
  }
  if (length == 8 && memcmp(payload, "WAKE_RST", 8) == 0) {
    mqttClient.publish(topic_sta, "OFFLINE", true);
    sendCommandWithCrc("RESET");
    IWatchdog.reload();
    delay(500); 
    IWatchdog.reload();
    NVIC_SystemReset(); 
    return;
  }
  if (length == 9 && memcmp(payload, "RST_SLAVE", 9) == 0) {
    sendCommandWithCrc("RESET");
    return;
  }
  if (!isSystemSleeping && length == 2) {
    uint16_t mask = ((uint16_t)payload[0] << 8) | payload[1];
    for (int i = 0; i < 7; i++) digitalWrite(outPins[i], (mask & (1 << i)) ? HIGH : LOW);
    for (int i = 0; i < 7; i++) lastSlaveState[i] = (mask & (1 << (i + 7)));
    syncSlave();
    safetyShutdownActive = false;
  }
}

void setup() {
  IWatchdog.begin(20000000);
  analogReadResolution(10);

  SlaveSerial.begin(9600); 
  pinMode(CMD_PIN, OUTPUT); digitalWrite(CMD_PIN, LOW); 
  
  // Hardware Reset W5500
  pinMode(W5500_RST, OUTPUT); 
  digitalWrite(W5500_RST, LOW); delay(50); 
  IWatchdog.reload();
  digitalWrite(W5500_RST, HIGH); delay(200);
  IWatchdog.reload();
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
      IWatchdog.reload();
    }
  }
  SPI.setMOSI(W5500_MOSI); SPI.setMISO(W5500_MISO); SPI.setSCLK(W5500_SCK); SPI.begin();
  Ethernet.init(W5500_CS);
  Ethernet.begin(mac, ip, myDns, gateway, subnet); 
  
  if (Ethernet.hardwareStatus() == EthernetNoHardware) {
    while (true) {
      digitalWrite(LED_PIN, !digitalRead(LED_PIN));
      delay(50); 
      IWatchdog.reload();
    }
  }

  mqttClient.setBufferSize(512);
  mqttClient.setServer(mqtt_broker, 1883);
  mqttClient.setCallback(mqttCallback);
  
  syncSlave();
  lastConnectionTime = millis();
}

void loop() {
  IWatchdog.reload(); 
  unsigned long now = millis();

  if (now - lastHeartbeat > 2000) {
      lastHeartbeat = now;
      syncSlave(); 
  }

  if (!mqttClient.connected()) {
    if (Ethernet.linkStatus() == LinkON) {
        if (now - lastMqttRetry > 5000) {
          lastMqttRetry = now;
          
          IWatchdog.reload(); 
          
          if (mqttClient.connect(serial_numb, mqtt_user, mqtt_pass, topic_sta, 1, true, "OFFLINE")) {
            IWatchdog.reload();
            mqttClient.subscribe(topic_sub);
            mqttClient.publish(topic_sta, isSystemSleeping ? "SLEEP" : "ONLINE", true);
            lastConnectionTime = now;
            // Note: NOT clearing safetyShutdownActive - let MQTT callback do it
            syncSlave();
          }
          IWatchdog.reload();
        }
    } else {
       lastMqttRetry = now;
    }
    
    // SAFETY SHUTDOWN: Turn off outputs after 30s
    if (now - lastConnectionTime > SAFETY_SHUTDOWN_TIMEOUT && !safetyShutdownActive) {
        safetyShutdown();
    }
    
    // REBOOT STRATEGY: Last resort recovery after 5 minutes
    if (now - lastConnectionTime > REBOOT_TIMEOUT) {
        IWatchdog.reload();
        NVIC_SystemReset();
    }
  } else {
    mqttClient.loop();
    lastConnectionTime = now;
  }

  if (now - lastPublish > 2000) {
    lastPublish = now;
    if (mqttClient.connected()) {
      publishData();
    }
  }

  static char rxBuf[128];
  static int rxIdx = 0;
  int rxCount = 0;
  while (SlaveSerial.available() && rxCount < 50) {  // Limit to prevent starving watchdog
    rxCount++;
    char c = SlaveSerial.read();
    if (c == '$') {
      rxIdx = 0; 
    }
    else if (c == '\n' || c == '\r') {
      if (rxIdx > 0 && rxIdx < 128) {
        rxBuf[rxIdx] = 0;
        handleSlaveData(rxBuf);
      }
      rxIdx = 0;
    } 
    else if (rxIdx < 127) {
      rxBuf[rxIdx++] = c;
    } else {
      rxIdx = 0;
    }
  }
  IWatchdog.reload();
}
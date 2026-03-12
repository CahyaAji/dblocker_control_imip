// SLAVE (STM32F401CCU6) v2.4 - FINAL
#include <IWatchdog.h>

// Config ========================
const bool USE_RS485 = false; 

#define LED_PIN PC13
#define CMD_PIN PA0 

HardwareSerial CmdSerial(PA10, PA9); 

uint32_t outPins[7] = { PB10, PB12, PA8, PB6, PB7, PB8, PB9 };
uint32_t hallSensorPins[9] = { PB1, PB0, PA7, PA6, PA5, PA4, PA3, PA2, PA1 };
int allHallSensors[9];

bool isSleeping = false;
unsigned long lastValidPacket = 0;

const unsigned long TIMEOUT_MS = 25000; 

uint8_t crc8(const char* data) {
  uint8_t crc = 0;
  while (*data) { crc ^= (uint8_t)(*data++); }
  return crc;
}

bool isHexChar(char c) {
  return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f');
}

bool parseSetCommand(const char* ptr, uint8_t outVals[7]) {
  for (int i = 0; i < 7; i++) {
    if (ptr[0] != '0' && ptr[0] != '1') return false;
    outVals[i] = (ptr[0] == '1') ? 1 : 0;
    ptr++;

    if (i < 6) {
      if (*ptr != ',') return false;
      ptr++;
    }
  }
  return *ptr == '\0';
}

void sendToMaster(const char* data) {
  if (USE_RS485) {
    digitalWrite(CMD_PIN, HIGH);
    delayMicroseconds(50);
  }
  
  CmdSerial.print(data);
  CmdSerial.flush(); 
  
  if (USE_RS485) {
    delay(2);
    digitalWrite(CMD_PIN, LOW);
  }
}

void sendResponseWithCrc(const char* payload) {
  uint8_t crc = crc8(payload);
  char fullPacket[128]; 
  snprintf(fullPacket, sizeof(fullPacket), "$%s|%s%X\r\n",
           payload, (crc < 0x10 ? "0" : ""), crc);
  sendToMaster(fullPacket);
}

void replyToMaster() {
  if (isSleeping) {
    sendResponseWithCrc("STA:SLEEP");
    return;
  }
  
  for(int i=0; i<9; i++) {
     analogRead(hallSensorPins[i]); 
     allHallSensors[i] = analogRead(hallSensorPins[i]);
  }

  static char payload[128];
  int offset = snprintf(payload, sizeof(payload), "CUR:");
  for(int i=0; i<9; i++) {
    offset += snprintf(payload + offset, sizeof(payload) - offset, "%d%s", 
                       allHallSensors[i], (i < 8) ? "," : "");
  }
  
  sendResponseWithCrc(payload);
}

void failsafeShutdown() {
  if (isSleeping) return; 
  isSleeping = true;
  for(int i=0; i<7; i++) digitalWrite(outPins[i], LOW);
}

void processCommand(char* cmd) {
  if (strcmp(cmd, "SLEEP") == 0) {
    failsafeShutdown();
    return;
  }

  if (strcmp(cmd, "WAKE") == 0) {
    isSleeping = false;
    replyToMaster();
    return;
  }

  if (strcmp(cmd, "RESET") == 0) {
    digitalWrite(LED_PIN, LOW);
    delay(100);
    digitalWrite(LED_PIN, HIGH);
    delay(100);
    NVIC_SystemReset();
    return;
  }

  if (strncmp(cmd, "SET:", 4) == 0) {
    uint8_t parsedVals[7];
    if (parseSetCommand(cmd + 4, parsedVals)) {
      isSleeping = false;
      for (int i = 0; i < 7; i++) {
        digitalWrite(outPins[i], parsedVals[i] ? HIGH : LOW);
      }
      replyToMaster();
    }
    return;
  }

  if (strcmp(cmd, "REQ") == 0 || strncmp(cmd, "REQ:", 4) == 0) {
      replyToMaster();
  }
}

void verifyAndExecute(char* buf) {
    char* pipePtr = strchr(buf, '|');
    if (!pipePtr) return; 

    *pipePtr = 0; 
    char* payload = buf;
    char* crcHex = pipePtr + 1; 

    if (strlen(crcHex) != 2) return;
    if (!isHexChar(crcHex[0]) || !isHexChar(crcHex[1])) return;

    if (crc8(payload) == (uint8_t) strtol(crcHex, NULL, 16)) {
        lastValidPacket = millis();
        processCommand(payload);
    }
}

void setup(){
  delay(1000); 
  
  analogReadResolution(10); 

  CmdSerial.begin(9600);
  
  pinMode(CMD_PIN, OUTPUT); digitalWrite(CMD_PIN, LOW); 
  pinMode(LED_PIN, OUTPUT);

  for (int i = 0; i < 7; i++) {
    pinMode(outPins[i], OUTPUT);
    digitalWrite(outPins[i], LOW);
  }
  
  IWatchdog.begin(10000000); 
  lastValidPacket = millis(); 

  sendResponseWithCrc("REQ:SYNC");
}

void loop(){
  IWatchdog.reload(); 

  static char rxBuf[128]; 
  static int rxIdx = 0;

  while (CmdSerial.available()) {
    char c = CmdSerial.read();
    
    if (c == '$') { 
      rxIdx = 0; 
      digitalWrite(LED_PIN, LOW); 
    } 
    else if (c == '\r' || c == '\n') {
      if (rxIdx > 0) {
        rxBuf[rxIdx] = 0; 
        verifyAndExecute(rxBuf); 
        rxIdx = 0;
      }
      digitalWrite(LED_PIN, HIGH); 
    } 
    else if (rxIdx < 127) { 
      rxBuf[rxIdx++] = c; 
    }
    else { 
      rxIdx = 0; 
    }
  }

  if (millis() - lastValidPacket > TIMEOUT_MS) {
      failsafeShutdown();
  }
}
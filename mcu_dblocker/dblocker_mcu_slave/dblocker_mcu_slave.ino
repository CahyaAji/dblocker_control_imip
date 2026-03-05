// SLAVE (STM32F401CCU6)
#include <IWatchdog.h>

// Config ========================
const bool USE_RS485 = false; 

#define LED_PIN PC13
#define CMD_PIN PA0 

HardwareSerial CmdSerial(PA10, PA9); 

uint32_t outPins[7] = { PB10, PB12, PA8, PB6, PB7, PB8, PB9 };
// uint32_t outPins[7] = { PB8, PB7, PB6, PA8, PB12, PB10, PB9 };
uint32_t hallSensorPins[9] = { PA1, PA2, PA3, PA4, PA5, PA6, PA7, PB0, PB1 };
int allHallSensors[9];

bool isSleeping = false;
unsigned long lastValidPacket = 0;
const unsigned long TIMEOUT_MS = 10000;

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

void replyToMaster() {
  // Stability: Double read to settle ADC
  for(int i=0; i<9; i++) {
     analogRead(hallSensorPins[i]); 
     allHallSensors[i] = analogRead(hallSensorPins[i]);
    IWatchdog.reload();
  }

  if (USE_RS485) {
      delayMicroseconds(500); 
      digitalWrite(CMD_PIN, HIGH); 
      delayMicroseconds(50);
  }

  CmdSerial.print("CUR:");
  for(int i=0; i<9; i++) {
    CmdSerial.print(allHallSensors[i]);
    if(i < 8) CmdSerial.print(",");
  }
  CmdSerial.println();
  
  CmdSerial.flush(); 

  if (USE_RS485) {
      delayMicroseconds(500); 
      digitalWrite(CMD_PIN, LOW);
  }
}

void failsafeShutdown() {
  if (!isSleeping) {
     for(int i=0; i<7; i++) digitalWrite(outPins[i], LOW);
  }
}

void processCommand(char* cmd) {
  lastValidPacket = millis();

  if (strcmp(cmd, "SLEEP") == 0) {
    isSleeping = true;
    for(int i=0; i<7; i++) digitalWrite(outPins[i], LOW);
    return;
  }

  if (strcmp(cmd, "WAKE") == 0) {
    isSleeping = false;
    replyToMaster();
    return;
  }

  if (strcmp(cmd, "RESET") == 0) {
    IWatchdog.reload();
    delay(100);
    IWatchdog.reload();
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

  if (strncmp(cmd, "REQ", 3) == 0) {
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
        processCommand(payload);
    }
}

void setup(){
  delay(100); 
  
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

  if (USE_RS485) { digitalWrite(CMD_PIN, HIGH); delay(2); }
  CmdSerial.println("REQ:SYNC");
  CmdSerial.flush();
  if (USE_RS485) { delayMicroseconds(500); digitalWrite(CMD_PIN, LOW); }
}

void loop(){
  IWatchdog.reload(); 

  static char rxBuf[64];
  static int rxIdx = 0;
  int rxCount = 0;

  while (CmdSerial.available() && rxCount < 50) {
    rxCount++;
    char c = CmdSerial.read();
    if (c == '$') { rxIdx = 0; digitalWrite(LED_PIN, LOW); } 
    else if (c == '\r' || c == '\n') {
      if (rxIdx > 0) {
        rxBuf[rxIdx] = 0; 
        verifyAndExecute(rxBuf); 
        rxIdx = 0;
      }
      digitalWrite(LED_PIN, HIGH); 
    } 
    else if (rxIdx < 63) { rxBuf[rxIdx++] = c; }
    else { rxIdx = 0; }
  }

  IWatchdog.reload();

  if (millis() - lastValidPacket > TIMEOUT_MS) {
      failsafeShutdown();
  }
}
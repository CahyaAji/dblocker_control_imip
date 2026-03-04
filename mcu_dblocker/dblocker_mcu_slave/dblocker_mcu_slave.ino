// SLAVE (STM32F401CCU6)
#include <IWatchdog.h>

// --- CONFIGURATION ---
const bool USE_RS485 = false; 

#define LED_PIN PC13
#define CMD_PIN PA0 

HardwareSerial CmdSerial(PA10, PA9); 

uint32_t outPins[7] = { PB8, PB7, PB6, PA8, PB12, PB10, PB9 };
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

void replyToMaster() {
  // Stability: Double read to settle ADC
  for(int i=0; i<9; i++) {
     analogRead(hallSensorPins[i]); 
     allHallSensors[i] = analogRead(hallSensorPins[i]);
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

  if (strstr(cmd, "SLEEP")) {
    isSleeping = true;
    for(int i=0; i<7; i++) digitalWrite(outPins[i], LOW);
    return;
  }

  if (strstr(cmd, "WAKE")) {
    isSleeping = false;
    replyToMaster();
    return;
  }

  if (strstr(cmd, "RESET")) {
    delay(100);
    NVIC_SystemReset();
    return;
  }

  // OPTIMIZED: Replaced heavy sscanf with lightweight parsing
  // Command format: "SET:1,0,1,0,1,0,1"
  if (strncmp(cmd, "SET:", 4) == 0) {
    char* ptr = cmd + 4; // Skip "SET:"
    isSleeping = false;
    
    for(int i=0; i<7; i++) {
       if (*ptr == '\0') break; // Safety
       
       // Parse single digit (0 or 1)
       int val = (*ptr == '1') ? 1 : 0;
       digitalWrite(outPins[i], val ? HIGH : LOW);
       
       // Move to next number (skip current char and comma)
       ptr++; 
       if (*ptr == ',') ptr++;
    }
    replyToMaster();
  }

  if (strstr(cmd, "REQ")) {
      replyToMaster();
  }
}

void verifyAndExecute(char* buf) {
    char* pipePtr = strchr(buf, '|');
    if (!pipePtr) return; 

    *pipePtr = 0; 
    char* payload = buf;
    char* crcHex = pipePtr + 1; 

    if (crc8(payload) == (uint8_t) strtol(crcHex, NULL, 16)) {
        processCommand(payload);
    }
}

void setup(){
  // Power-on stability delay
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

  // Initial Sync Request
  if (USE_RS485) { digitalWrite(CMD_PIN, HIGH); delay(2); }
  CmdSerial.println("REQ:SYNC");
  CmdSerial.flush();
  if (USE_RS485) { delayMicroseconds(500); digitalWrite(CMD_PIN, LOW); }
}

void loop(){
  IWatchdog.reload(); 

  static char rxBuf[64];
  static int rxIdx = 0;

  while (CmdSerial.available()) {
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
  }

  if (millis() - lastValidPacket > TIMEOUT_MS) {
      failsafeShutdown();
  }
}
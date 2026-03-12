### MCU Notes
#### 1. CHIP
- master gunakan stm32f411
- slave gunakan stm32f401

#### 2. Protocol mqtt
##### a. sub
- Topic subcribe
```
dbl/[serial_numb]/cmd
```
- Format cmd
```
2-byte binary payload (14-bit bitmask)
[Byte 1 (8 bits)] [Byte 2 (6 bits used)]

Bit mapping (LSB first):
Bit 0-1   : Sector 0 [GPS, Ctrl]
Bit 2-3   : Sector 1 [GPS, Ctrl]
Bit 4-5   : Sector 2 [GPS, Ctrl]
Bit 6     : Fan Master
Bit 7-8   : Sector 3 [GPS, Ctrl]
Bit 9-10  : Sector 4 [GPS, Ctrl]
Bit 11-12 : Sector 5 [GPS, Ctrl]
Bit 13    : Fan Slave
```

- Example cmd
```
Topic: dbl/250001/cmd

Payload (Hex): 0x00 0x00
  All signals OFF: Sector 0-5 GPS/Ctrl disabled, Fan Master/Slave OFF

Payload (Hex): 0x01 0x00
  Sector 0 GPS ON: Only bit 0 set

Payload (Hex): 0x03 0x00
  Sector 0 GPS+Ctrl ON: Bits 0-1 set

Payload (Hex): 0x40 0x20
  All ON: All sectors and fans enabled
  (0x40 = bits 6 set, 0x20 = bit 13 set, plus bits 0-5, 7-12)
```

##### b. pub (status)
- Topic status
```
dbl/[serial_numb]/sta
```
- Format sta (retained)

| Payload | Description |
|---------|-------------|
| `OFF` | Device offline / disconnected (LWT) or after reset |
| `SLEEP` | Device online but in sleep state (all outputs OFF) |
| `ON:XXXX` | Device online and active; XXXX = 4-digit hex bitmask of outpin states |

- Bitmask in `ON:XXXX`
```
Bits 0-6  : Master outpins 0-6
Bits 7-13 : Slave outpins 0-6
```

- Example sta
```
OFF           → device disconnected
SLEEP         → device connected, all outputs disabled
ON:0000       → device active, all outputs OFF
ON:0001       → device active, master pin 0 ON
ON:0003       → device active, master pins 0+1 ON
ON:3FFF       → device active, all 14 outputs ON
```
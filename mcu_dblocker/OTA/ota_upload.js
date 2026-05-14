const dgram = require('dgram');
const net = require('net');
const fs = require('fs');

// --- CONFIGURATION ---
const TARGET_IP = '10.88.81.10'; // Updated to your new IP
const UDP_PORT = 51515;
const TCP_PORT = 8080;
const UDP_SECRET = 'p!ml_3rUc35';
const UDP_TIMEOUT_MS = 3000;

const firmwareFile = process.argv[2] || 'firmware.bin';

if (!fs.existsSync(firmwareFile)) {
    console.error(`❌ Error: Firmware file '${firmwareFile}' not found!`);
    process.exit(1);
}

// 1. Calculate exact file size for the safety handshake
const fileSize = fs.statSync(firmwareFile).size;

function getCrc8Hex(str) {
    let crc = 0;
    for (let i = 0; i < str.length; i++) {
        crc ^= str.charCodeAt(i); 
    }
    return crc.toString(16).toUpperCase().padStart(2, '0'); 
}

// 2. Build the secure UDP packet WITH the file size attached
const payload = `${UDP_SECRET}:OTA_START:${fileSize}`;
const crcHex = getCrc8Hex(payload);
const fullPacket = `${payload}|${crcHex}`;

console.log(`\n🚀 STM32 Firmware OTA Uploader (Failsafe Edition)`);
console.log(`--------------------------------------------------`);
console.log(`📦 Firmware : ${firmwareFile} (${fileSize} bytes)`);
console.log(`🎯 Target   : ${TARGET_IP}`);

const udpClient = dgram.createSocket('udp4');
let timeoutHandle;

udpClient.on('message', (msg) => {
    clearTimeout(timeoutHandle);
    const response = msg.toString().trim();
    console.log(`\n[UDP] 📥 Received: ${response}`);

    if (response.includes('ACK: OTA MODE READY')) {
        console.log(`[UDP] ✅ Controller expecting ${fileSize} bytes. Switching to TCP...`);
        udpClient.close();
        uploadFirmware(); 
    } else {
        console.error(`[UDP] ❌ Unexpected response. Aborting.`);
        udpClient.close();
        process.exit(1);
    }
});

console.log(`[UDP] 📤 Sending Trigger: ${fullPacket}`);
udpClient.send(fullPacket, UDP_PORT, TARGET_IP, (err) => {
    if (err) {
        console.error(`[UDP] ❌ Send error:`, err);
        udpClient.close();
        process.exit(1);
    }
});

timeoutHandle = setTimeout(() => {
    console.error(`[UDP] ⏱️ Timeout waiting for ACK. Is the board online?`);
    udpClient.close();
    process.exit(1);
}, UDP_TIMEOUT_MS);

function uploadFirmware() {
    console.log(`\n[TCP] 🔌 Connecting to ${TARGET_IP}:${TCP_PORT}...`);
    const tcpClient = new net.Socket();

    tcpClient.connect(TCP_PORT, TARGET_IP, () => {
        console.log(`[TCP] 🟢 Connected! Sending firmware...`);
        
        const fileData = fs.readFileSync(firmwareFile);
        
        tcpClient.write(fileData, () => {
            console.log(`[TCP] 📤 Transfer complete! Telling controller to verify bytes...`);
            tcpClient.end(); 
            
            setTimeout(() => {
                console.log(`[TCP] ⏱️ Controller disconnected to reboot. Success!`);
                tcpClient.destroy();
                process.exit(0);
            }, 2000);
        });
    });

    tcpClient.on('close', () => {
        console.log(`[TCP] 🏁 Connection closed.`);
        console.log(`🎉 Update finished! If bytes matched, STM32 will reboot now.\n`);
        process.exit(0);
    });

    tcpClient.on('error', (err) => {
        console.error(`[TCP] ❌ Socket Error:`, err.message);
        process.exit(1);
    });
}
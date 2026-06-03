/**
 * ============================================================================
 * 🚑 STM32 EMERGENCY RESCUE UPLOADER
 * ============================================================================
 *
 * WHEN TO USE THIS:
 * Only use this script if your board is "bricked" (frozen or stuck in a crash 
 * loop) because of bad code, and it no longer responds to the normal UDP trigger.
 *
 * HOW THE RESCUE WORKS:
 * When the STM32 is physically powered on, it waits exactly 5 seconds before 
 * running your main code. During those 5 seconds, it listens on TCP Port 8080.
 * This script aggressively knocks on that port twice a second until the board 
 * wakes up, opens the door, and accepts the new firmware.
 *
 * EXACT RESCUE STEPS:
 * 1. In the Arduino IDE, fix your broken code and click "Export Compiled Binary".
 * 2. Open your terminal and run this script targeting the fixed .bin file:
 *    
 *      node ota_rescue.js mcu_master_4.x.ino.bin
 *
 * 3. The terminal will say "Waiting..." and print dots. 
 * 4. Walk over to the STM32 board and press the physical 'RST' button OR 
 *    unplug its power and plug it back in.
 * 5. The script will instantly catch the board during its 5-second boot window,
 *    upload the good firmware, and bring it back to life!
 * ============================================================================
 */

const net = require('net');
const fs = require('fs');

// --- CONFIGURATION ---
const TARGET_IP = '10.88.81.3';
const TCP_PORT = 8080;
const RETRY_INTERVAL_MS = 500;

const firmwareFile = process.argv[2] || 'firmware.bin';

if (!fs.existsSync(firmwareFile)) {
    console.error(`❌ Error: Firmware file '${firmwareFile}' not found!`);
    process.exit(1);
}

const fileData = fs.readFileSync(firmwareFile);
const fileSize = fileData.length;

console.log(`\n🚑 STM32 EMERGENCY RESCUE UPLOADER`);
console.log(`-----------------------------------`);
console.log(`📦 Rescue Firmware : ${firmwareFile} (${fileSize} bytes)`);
console.log(`🎯 Target          : ${TARGET_IP}:${TCP_PORT}`);
console.log(`\n⏳ Waiting for the STM32 to reboot... (Press Ctrl+C to cancel)`);

function attemptConnection() {
    process.stdout.write('.'); // Print dots to show it is actively trying

    const tcpClient = new net.Socket();
    
    // Silence error crashes (we EXPECT connection refused errors while the board is dead/rebooting)
    tcpClient.on('error', () => {
        tcpClient.destroy();
        setTimeout(attemptConnection, RETRY_INTERVAL_MS);
    });

    tcpClient.connect(TCP_PORT, TARGET_IP, () => {
        console.log(`\n\n[TCP] 🟢 RESCUE DOOR OPEN! Sending firmware...`);
        
        tcpClient.write(fileData, () => {
            console.log(`[TCP] 📤 Transfer complete! Telling controller to apply...`);
            tcpClient.end(); 
            
            setTimeout(() => {
                console.log(`[TCP] ⏱️ Rescue finished! The STM32 should boot safely now.\n`);
                tcpClient.destroy();
                process.exit(0);
            }, 2000);
        });
    });
}

// Start the continuous knocking
attemptConnection();
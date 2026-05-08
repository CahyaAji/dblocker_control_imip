#!/usr/bin/env node
/**
 * CAMERA SIMULATOR — Hikvision ISAPI mock server
 *
 * Simulates camera PTZ, status, and snapshot endpoints.
 * All requests are printed in a readable, colour-coded format.
 * Auth is intentionally skipped (accepts any credentials).
 *
 * Usage:
 *   node camera_simulator.js [--port=8080] [--channel=1] [--id=CAM-SIM]
 *
 * To run multiple cameras on different ports:
 *   node camera_simulator.js --port=8081 --id=CAM-01
 *   node camera_simulator.js --port=8082 --id=CAM-02
 */

'use strict';
const http = require('http');

// ─── CLI args ─────────────────────────────────────────────────────────────────
function arg(name, fallback) {
  const entry = process.argv.find(a => a.startsWith(`--${name}=`));
  return entry ? entry.split('=').slice(1).join('=') : fallback;
}
const PORT    = parseInt(arg('port',    '8800'), 10);
const CHANNEL = parseInt(arg('channel', '1'),    10);
const CAM_ID  = arg('id', 'CAM-SIM');

// ─── Movement config ──────────────────────────────────────────────────────────
const PAN_DEG_PER_SEC  = 30;  // degrees/s at speed 100
const TILT_DEG_PER_SEC = 15;  // degrees/s at speed 100
const ZOOM_STEP_PER_SEC = 20; // zoom units/s at speed 100
const TICK_MS          = 50;  // simulation interval

// ─── State ────────────────────────────────────────────────────────────────────
let azimuth   = 0;   // ISAPI tenths-of-degrees (0–3599)
let elevation = 0;   // ISAPI tenths-of-degrees (negative = down)
let zoom      = 100; // absolute zoom (100 = 1×)
let panSpeed  = 0;   // active continuous speeds (-100..100)
let tiltSpeed = 0;
let zoomSpeed = 0;
let moveTicker = null;

function startMoving() {
  if (moveTicker) return;
  moveTicker = setInterval(() => {
    const dt = TICK_MS / 1000;
    if (panSpeed !== 0) {
      const delta = (panSpeed / 100) * PAN_DEG_PER_SEC * dt * 10;
      azimuth = Math.round(((azimuth + delta) % 3600 + 3600) % 3600);
    }
    if (tiltSpeed !== 0) {
      const delta = (tiltSpeed / 100) * TILT_DEG_PER_SEC * dt * 10;
      elevation = Math.round(Math.max(-900, Math.min(900, elevation + delta)));
    }
    if (zoomSpeed !== 0) {
      const delta = (zoomSpeed / 100) * ZOOM_STEP_PER_SEC * dt;
      zoom = Math.round(Math.max(10, Math.min(300, zoom + delta)));
    }
  }, TICK_MS);
}

function stopMoving() {
  if (moveTicker) { clearInterval(moveTicker); moveTicker = null; }
}

// ─── ANSI colours ─────────────────────────────────────────────────────────────
const C = {
  reset:   '\x1b[0m',
  bold:    '\x1b[1m',
  dim:     '\x1b[2m',
  gray:    '\x1b[90m',
  cyan:    '\x1b[96m',
  green:   '\x1b[92m',
  yellow:  '\x1b[93m',
  blue:    '\x1b[94m',
  magenta: '\x1b[95m',
  red:     '\x1b[91m',
};

function ts() {
  return new Date().toTimeString().slice(0, 8);
}

function log(color, label, details) {
  console.log(`${C.gray}[${ts()}]${C.reset} ${color}${C.bold}${label}${C.reset}  ${details}`);
}

// ─── Formatting helpers ───────────────────────────────────────────────────────
function fmtDeg(tenths) {
  return `${(tenths / 10).toFixed(1)}°`;
}

function fmtSpeed(v) {
  if (v > 0) return `${C.green}+${v}${C.reset}`;
  if (v < 0) return `${C.red}${v}${C.reset}`;
  return `${C.dim}0${C.reset}`;
}

function fmtDir(pan, tilt) {
  const h = pan > 0 ? '→' : pan < 0 ? '←' : '·';
  const v = tilt > 0 ? '↑' : tilt < 0 ? '↓' : '·';
  return `${h}${v}`;
}

// ─── XML helpers ──────────────────────────────────────────────────────────────
function xmlNum(body, tag) {
  const m = body.match(new RegExp(`<${tag}>\\s*(-?\\d+)\\s*<\\/${tag}>`));
  return m ? parseInt(m[1], 10) : null;
}

function responseOK(url) {
  return (
    '<?xml version="1.0" encoding="UTF-8"?>\n' +
    '<ResponseStatus>\n' +
    `  <requestURL>${url}</requestURL>\n` +
    '  <statusCode>1</statusCode>\n' +
    '  <statusString>OK</statusString>\n' +
    '  <subStatusCode>ok</subStatusCode>\n' +
    '</ResponseStatus>'
  );
}

function ptzStatusXML() {
  return (
    '<?xml version="1.0" encoding="UTF-8"?>\n' +
    '<PTZStatus>\n' +
    '  <AbsoluteHigh>\n' +
    `    <elevation>${elevation}</elevation>\n` +
    `    <azimuth>${azimuth}</azimuth>\n` +
    `    <absoluteZoom>${zoom}</absoluteZoom>\n` +
    '  </AbsoluteHigh>\n' +
    '</PTZStatus>'
  );
}

// ─── Tiny 1×1 placeholder JPEG ────────────────────────────────────────────────
// A valid minimal JPEG so the vision server doesn't choke on the response.
const PLACEHOLDER_JPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8U' +
  'HRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAARCAABAAEDASIA' +
  'AhEBAxEB/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/xAAU' +
  'AQEAAAAAAAAAAAAAAAAAAAAA/8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEQMRAD8A' +
  'JQAB/9k=',
  'base64'
);

// ─── Body reader ──────────────────────────────────────────────────────────────
function readBody(req) {
  return new Promise(resolve => {
    const chunks = [];
    req.on('data', c => chunks.push(c));
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
  });
}

// ─── Request router ───────────────────────────────────────────────────────────
async function handle(req, res) {
  const url    = req.url.split('?')[0];
  const method = req.method.toUpperCase();
  const ch     = CHANNEL;

  // ── Absolute PTZ  PUT /ISAPI/PTZCtrl/channels/{ch}/absolute ──────────────
  if (method === 'PUT' && url === `/ISAPI/PTZCtrl/channels/${ch}/absolute`) {
    const body = await readBody(req);
    const az   = xmlNum(body, 'azimuth');
    const el   = xmlNum(body, 'elevation');
    const zz   = xmlNum(body, 'absoluteZoom');

    stopMoving();
    panSpeed = tiltSpeed = zoomSpeed = 0;

    if (az != null) azimuth   = ((az % 3600) + 3600) % 3600;
    if (el != null) elevation = Math.max(-900, Math.min(900, el));
    if (zz != null && zz > 0) zoom = Math.max(10, Math.min(300, zz));

    if (az != null || el != null) {
      log(C.cyan, 'PTZ ABSOLUTE',
        `az: ${String(azimuth).padStart(4)} (${fmtDeg(azimuth).padStart(7)})` +
        `  el: ${String(elevation).padStart(4)} (${fmtDeg(elevation).padStart(7)})`);
    }
    if (zz != null && zz > 0) {
      log(C.cyan, 'ZOOM ABSOLUTE', `zoom: ${zoom}`);
    }

    res.writeHead(200, { 'Content-Type': 'application/xml' });
    res.end(responseOK(url));
    return;
  }

  // ── Continuous PTZ  PUT /ISAPI/PTZCtrl/channels/{ch}/continuous ───────────
  if (method === 'PUT' && url === `/ISAPI/PTZCtrl/channels/${ch}/continuous`) {
    const body = await readBody(req);
    const pan  = xmlNum(body, 'pan')  ?? 0;
    const tilt = xmlNum(body, 'tilt') ?? 0;
    const zs   = xmlNum(body, 'zoom') ?? 0;

    panSpeed  = pan;
    tiltSpeed = tilt;
    zoomSpeed = zs;

    if (pan === 0 && tilt === 0 && zs === 0) {
      stopMoving();
      log(C.yellow, 'PTZ STOP    ',
        `→ az: ${String(azimuth).padStart(4)} (${fmtDeg(azimuth).padStart(7)})` +
        `  el: ${String(elevation).padStart(4)} (${fmtDeg(elevation).padStart(7)})`);
    } else {
      startMoving();
      if (pan !== 0 || tilt !== 0) {
        log(C.green, 'PTZ MOVE    ',
          `pan: ${fmtSpeed(pan)}  tilt: ${fmtSpeed(tilt)}  ${fmtDir(pan, tilt)}`);
      }
      if (zs !== 0) {
        log(C.green, 'ZOOM MOVE   ', `speed: ${fmtSpeed(zs)}`);
      }
    }

    res.writeHead(200, { 'Content-Type': 'application/xml' });
    res.end(responseOK(url));
    return;
  }

  // ── PTZ Status  GET /ISAPI/PTZCtrl/channels/{ch}/status ──────────────────
  if (method === 'GET' && url === `/ISAPI/PTZCtrl/channels/${ch}/status`) {
    log(C.blue, 'PTZ STATUS  ',
      `az: ${String(azimuth).padStart(4)} (${fmtDeg(azimuth).padStart(7)})` +
      `  el: ${String(elevation).padStart(4)} (${fmtDeg(elevation).padStart(7)})` +
      `  zoom: ${zoom}`);

    res.writeHead(200, { 'Content-Type': 'application/xml' });
    res.end(ptzStatusXML());
    return;
  }

  // ── Snapshot  GET /ISAPI/Streaming/channels/{ch}*/picture ────────────────
  if (method === 'GET' && url.includes(`/ISAPI/Streaming/channels/${ch}`)) {
    log(C.magenta, 'SNAPSHOT    ', `placeholder JPEG (1×1)`);
    res.writeHead(200, {
      'Content-Type':   'image/jpeg',
      'Content-Length': PLACEHOLDER_JPEG.length,
    });
    res.end(PLACEHOLDER_JPEG);
    return;
  }

  // ── Unknown ───────────────────────────────────────────────────────────────
  log(C.red, 'UNKNOWN     ', `${method} ${url}`);
  res.writeHead(404, { 'Content-Type': 'text/plain' });
  res.end('Not Found\n');
}

// ─── Server startup ───────────────────────────────────────────────────────────
const server = http.createServer((req, res) => {
  handle(req, res).catch(err => {
    console.error(`${C.red}[ERROR]${C.reset} ${err.message}`);
    res.writeHead(500);
    res.end('Internal Server Error\n');
  });
});

server.listen(PORT, () => {
  console.log('');
  console.log(`${C.bold}=== Hikvision Camera Simulator ===${C.reset}`);
  console.log(`  ID       : ${C.bold}${CAM_ID}${C.reset}`);
  console.log(`  Channel  : ${CHANNEL}`);
  console.log(`  Port     : ${C.bold}${PORT}${C.reset}`);
  console.log(`  Auth     : ${C.dim}skipped (accepts any credentials)${C.reset}`);
  console.log('');
  console.log('  Endpoints:');
  console.log(`    ${C.cyan}PUT${C.reset}  /ISAPI/PTZCtrl/channels/${CHANNEL}/absolute`);
  console.log(`    ${C.green}PUT${C.reset}  /ISAPI/PTZCtrl/channels/${CHANNEL}/continuous`);
  console.log(`    ${C.blue}GET${C.reset}  /ISAPI/PTZCtrl/channels/${CHANNEL}/status`);
  console.log(`    ${C.magenta}GET${C.reset}  /ISAPI/Streaming/channels/${CHANNEL}01/picture`);
  console.log('');
  console.log(`  Waiting for requests ...`);
  console.log('');
});

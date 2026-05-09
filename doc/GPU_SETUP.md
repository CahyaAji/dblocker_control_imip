# GPU Acceleration Setup (NVIDIA)

Enable YOLO detection on an NVIDIA GPU instead of CPU.  
The switch is a single overlay file (`docker-compose.gpu.yml`) — no code changes needed.

| Machine | Command |
|---|---|
| Your laptop (CPU) | `docker compose -f docker-compose.prod.yml up -d --build` |
| Dell Precision 5860 (GPU) | `docker compose -f docker-compose.prod.yml -f docker-compose.gpu.yml up -d --build` |

---

## Prerequisites

### 1. NVIDIA driver

The driver must already be installed on the host machine (not inside Docker).

```bash
# Verify driver is installed and GPU is visible
nvidia-smi
```

You should see a table showing your GPU model and driver version.  
If `nvidia-smi` is not found, install the driver first:

```bash
# Ubuntu / Debian
sudo apt-get install -y nvidia-driver-535   # or latest available
sudo reboot
```

### 2. nvidia-container-toolkit

This allows Docker containers to access the GPU.

```bash
# Add NVIDIA package repository
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
  | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg

curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
  | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
  | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

# Install
sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit

# Restart Docker so it picks up the new runtime
sudo systemctl restart docker
```

### 3. Verify Docker can see the GPU

```bash
docker run --rm --gpus all nvidia/cuda:12.4.1-base-ubuntu24.04 nvidia-smi
```

You should see the same GPU table as before. If this works, you are ready.

---

## Deploy on Dell Precision 5860

### Step 1 — Clone / pull the repository

```bash
git clone <repo-url> dblocker_control_imip
cd dblocker_control_imip
```

Or if already cloned:

```bash
git pull
```

### Step 2 — Copy and fill the env file

```bash
cp .env.example .env.prod   # if an example exists, otherwise create it manually
nano .env.prod
```

Minimum GPU-relevant variable to add:

```env
# Leave this out for CPU, add it for GPU
DETECT_DEVICE=cuda
```

All other variables (`CAMERA_USERNAME`, `CAMERA_PASSWORD`, `DB_*`, etc.) are the same as the regular production deployment — see [DEPLOYMENT.md](DEPLOYMENT.md).

> `DETECT_DEVICE` is automatically set by `docker-compose.gpu.yml`, so you do **not** need to add it to `.env.prod`. It is listed here only for reference.

### Step 3 — Build and start with GPU overlay

```bash
docker compose --env-file .env.prod \
  -f docker-compose.prod.yml \
  -f docker-compose.gpu.yml \
  up -d --build
```

The build will:
1. Compile the Go binary (same as CPU).
2. Pull `nvidia/cuda:12.4.1-cudnn-runtime-ubuntu24.04` as the runtime base image.
3. Download the **GPU** ONNX Runtime release (`onnxruntime-linux-x64-gpu-1.25.1.tgz`).

First build takes longer because of the larger base image (~4 GB download). Subsequent builds are cached.

### Step 4 — Verify GPU is being used

Check the vision container logs:

```bash
docker compose -f docker-compose.prod.yml logs --tail=30 vision
```

You should see a line like:

```
Detector: CUDA execution provider enabled (GPU)
Detector loaded: /models/yolov8n.onnx (conf=0.35, iou=0.45)
```

If you see `Detector: using CPU execution provider` instead, the `DETECT_DEVICE=cuda` env var was not passed — double-check that you included `-f docker-compose.gpu.yml`.

### Step 5 — Confirm inference speed

Open the camera page in the browser and enable the **YOLO** toggle.  
On CPU: annotated frames lag visibly (~100–200 ms per frame).  
On GPU: frames are near real-time (~5–15 ms per frame on YOLOv8n).

---

## Switching back to CPU

Simply restart without the GPU overlay:

```bash
docker compose --env-file .env.prod \
  -f docker-compose.prod.yml \
  up -d --build
```

This rebuilds the vision image with the CPU ORT build and no CUDA EP.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `Failed to create CUDA provider options` in logs | ORT GPU build not used — `USE_GPU=true` build arg was not passed | Make sure `-f docker-compose.gpu.yml` is in the command |
| `could not select device driver "" with capabilities: [[gpu]]` | `nvidia-container-toolkit` not installed or Docker not restarted | Re-run Step 2 above and `sudo systemctl restart docker` |
| `nvidia-smi` works on host but not inside container | Old Docker / container-toolkit mismatch | `sudo apt-get upgrade nvidia-container-toolkit && sudo systemctl restart docker` |
| Container starts but GPU utilization stays at 0% | Model is small (YOLOv8n) — GPU launches are amortized; normal | Try a larger model (`yolov8s.onnx`) if you need to confirm |
| Vision crashes on first frame | cuDNN version mismatch with ORT 1.25.1 | Ensure driver ≥ 525; the `nvidia/cuda:12.4.1-cudnn-runtime` image requires CUDA 12.4 compatible driver |

---

## Environment variables reference

| Variable | Default | Description |
|---|---|---|
| `DETECT_DEVICE` | _(unset = CPU)_ | Set to `cuda` to use NVIDIA GPU |
| `DETECT_MODEL_PATH` | `/models/yolov8n.onnx` | Path to ONNX model inside container |
| `DETECT_CONF_THRESHOLD` | `0.35` | Minimum confidence to keep a detection |
| `DETECT_IOU_THRESHOLD` | `0.45` | IoU threshold for NMS |
| `DETECT_JPEG_QUALITY` | `75` | JPEG quality of annotated frames (1–100) |

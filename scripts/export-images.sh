#!/usr/bin/env bash
# Build images and export them as tar files for deployment to another machine.
# Output: dist/ folder containing tars + deploy package

set -e

DIST_DIR="./dist"
mkdir -p "$DIST_DIR"

echo "==> Building images..."
docker compose -f docker-compose.prod.yml build

echo "==> Exporting images..."
docker save dblocker-app:latest    | gzip > "$DIST_DIR/dblocker-app.tar.gz"
docker save dblocker-assist-app:latest | gzip > "$DIST_DIR/dblocker-assist.tar.gz"
docker save dblocker-vision:latest | gzip > "$DIST_DIR/dblocker-vision.tar.gz"

echo "==> Copying deploy files..."
cp docker-compose.deploy.yml "$DIST_DIR/"
cp -r mosquitto "$DIST_DIR/"
mkdir -p "$DIST_DIR/recordings"
mkdir -p "$DIST_DIR/models"
cp cmd/vision/models/yolov8n.onnx "$DIST_DIR/models/"

# Copy .env template if .env.example exists, otherwise copy .env without secrets hint
if [ -f ".env.example" ]; then
  cp .env.example "$DIST_DIR/.env.example"
fi

echo ""
echo "==> Done! Transfer the 'dist/' folder to the target machine."
echo "    Then on target: run scripts/load-images.sh"

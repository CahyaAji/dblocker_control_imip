#!/usr/bin/env bash
# Load pre-built images on the target (deployment) machine and start the stack.
# Run this from the directory containing docker-compose.deploy.yml

set -e

echo "==> Loading images..."
docker load -i dblocker-app.tar.gz
docker load -i dblocker-assist.tar.gz
docker load -i dblocker-vision.tar.gz

echo ""
echo "==> Images loaded. Next steps:"
echo "    1. Create a .env file with your secrets (copy .env.example if provided)"
echo "    2. Run: docker compose -f docker-compose.deploy.yml up -d"

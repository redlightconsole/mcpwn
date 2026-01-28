#!/bin/bash
set -e

APP="mcpwn"
VERSION="${1:-dev}"
DIST="dist"

rm -rf $DIST
mkdir -p $DIST/tmp

echo "Building ${APP} version ${VERSION}..."

TARGETS=("linux/amd64" "linux/arm64" "windows/amd64" "darwin/arm64")

for target in "${TARGETS[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "$target"
    OUT_NAME="${APP}-${GOOS}-${GOARCH}"
    BIN_NAME=$APP
    
    if [ "$GOOS" == "windows" ]; then BIN_NAME="${APP}.exe"; fi

    echo " -> Compiling for $GOOS/$GOARCH..."
    
    env GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=$(git rev-parse --short HEAD) -X main.Date=$(date +%F)" \
        -o "$DIST/tmp/$BIN_NAME" ./cmd/mcpwn/main.go

    cp mcpwn.yaml "$DIST/tmp/"
    
    cd "$DIST/tmp"
    if [ "$GOOS" == "windows" ]; then
        zip -q -r "../../$DIST/${OUT_NAME}.zip" .
    else
        tar -czf "../../$DIST/${OUT_NAME}.tar.gz" .
    fi
    cd - > /dev/null
    
    rm "${DIST}/tmp/${BIN_NAME}" "${DIST}/tmp/mcpwn.yaml"
done

rm -rf "${DIST}/tmp"
echo "[!] Build complete. Artifacts in ${DIST}/"
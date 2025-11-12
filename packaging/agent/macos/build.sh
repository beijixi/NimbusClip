#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DIST_DIR="${OUTPUT_DIR:-"$ROOT_DIR/dist/macos"}"
APP_NAME="clipflow-agent"
BIN_NAME="clipflow-agent"
UNIVERSAL_BIN="$DIST_DIR/$BIN_NAME"

mkdir -p "$DIST_DIR/tmp" "$DIST_DIR/artifacts"

build_arch() {
        local arch="$1"
        local out_dir="$DIST_DIR/tmp/$arch"
        mkdir -p "$out_dir"
        echo "Building $APP_NAME for darwin/$arch"
        local cgo="${CGO_ENABLED:-0}"
        CGO_ENABLED="$cgo" GOOS=darwin GOARCH="$arch" go build -o "$out_dir/$BIN_NAME" "$ROOT_DIR/cmd/agent"
}

build_arch amd64
build_arch arm64

if command -v lipo >/dev/null 2>&1; then
        echo "Creating universal binary"
        lipo -create "$DIST_DIR/tmp/amd64/$BIN_NAME" "$DIST_DIR/tmp/arm64/$BIN_NAME" -output "$UNIVERSAL_BIN"
else
        echo "lipo not available; keeping architecture-specific binaries"
        cp "$DIST_DIR/tmp/arm64/$BIN_NAME" "$UNIVERSAL_BIN-arm64"
        cp "$DIST_DIR/tmp/amd64/$BIN_NAME" "$UNIVERSAL_BIN-amd64"
fi

PAYLOAD_DIR="$DIST_DIR/payload"
mkdir -p "$PAYLOAD_DIR/usr/local/bin"
if [[ -f "$UNIVERSAL_BIN" ]]; then
        install -m 0755 "$UNIVERSAL_BIN" "$PAYLOAD_DIR/usr/local/bin/$BIN_NAME"
else
        install -m 0755 "$DIST_DIR/tmp/arm64/$BIN_NAME" "$PAYLOAD_DIR/usr/local/bin/$BIN_NAME"
fi

PKG_PATH="$DIST_DIR/artifacts/${APP_NAME}.pkg"
if command -v pkgbuild >/dev/null 2>&1; then
        echo "Packaging installer at $PKG_PATH"
        pkgbuild \
                --root "$PAYLOAD_DIR" \
                --identifier "com.clipflow.agent" \
                --version "${VERSION:-0.1.0}" \
                "$PKG_PATH"
else
        echo "pkgbuild not available; creating tarball instead"
        TAR_PATH="$DIST_DIR/artifacts/${APP_NAME}.tar.gz"
        (cd "$PAYLOAD_DIR" && tar -czf "$TAR_PATH" .)
        echo "Created archive $TAR_PATH"
        exit 0
fi

echo "Installer created at $PKG_PATH"
echo "TODO: Notarize and staple the package before distribution."

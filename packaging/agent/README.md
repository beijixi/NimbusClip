# Agent Packaging Guides

This directory contains helper scripts for producing distributable artifacts of the Clipflow agent.

## macOS

Use `packaging/agent/macos/build.sh` on a macOS machine with Xcode command line tools installed. The script builds arm64 and amd64 binaries and produces either a universal `.pkg` installer (when `pkgbuild` is available) or a `.tar.gz` archive as a fallback.

```bash
chmod +x packaging/agent/macos/build.sh
./packaging/agent/macos/build.sh
```

Set the `VERSION` environment variable to control the package version. After building, notarize and staple the installer before distributing.

## Windows

Run `packaging/agent/windows/build.ps1` in a PowerShell session on Windows. When the WiX Toolset (`candle.exe` and `light.exe`) is present in `PATH`, the script creates an `.msi` installer. Otherwise it falls back to a `.zip` archive containing the agent binary.

```powershell
powershell -ExecutionPolicy Bypass -File packaging/agent/windows/build.ps1
```

Provide the `VERSION` environment variable to label the build. Remember to code-sign the executable and installer before releasing.

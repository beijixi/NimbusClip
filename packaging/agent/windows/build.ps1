param(
    [string]$OutputDir = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Resolve-Path (Join-Path $scriptDir '..\\..\\..')
if (-not $OutputDir) {
    $OutputDir = Join-Path $rootDir 'dist\\windows'
}
$binDir = Join-Path $OutputDir 'bin'
$wixDir = Join-Path $OutputDir 'wix'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
New-Item -ItemType Directory -Force -Path $wixDir | Out-Null

$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '1'
$binaryPath = Join-Path $binDir 'clipflow-agent.exe'
Write-Host "Building Windows agent at $binaryPath"
& go build -o $binaryPath "$rootDir/cmd/agent"

$wxsPath = Join-Path $wixDir 'clipflow-agent.wxs'
@"
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="Clipflow Agent" Language="1033" Version="$(var.Version)" Manufacturer="Clipflow" UpgradeCode="5fdd9f92-43e2-4e2f-8b45-3d1e0dce01b1">
    <Package InstallerVersion="500" Compressed="yes" InstallScope="perMachine" />
    <MajorUpgrade AllowDowngrades="no" AllowSameVersionUpgrades="no" DowngradeErrorMessage="A newer version of Clipflow Agent is already installed." />
    <MediaTemplate />
    <Feature Id="ProductFeature" Title="Clipflow Agent" Level="1">
      <ComponentGroupRef Id="ProductComponents" />
    </Feature>
  </Product>
  <Fragment>
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFilesFolder">
        <Directory Id="INSTALLFOLDER" Name="Clipflow Agent" />
      </Directory>
    </Directory>
  </Fragment>
  <Fragment>
    <ComponentGroup Id="ProductComponents" Directory="INSTALLFOLDER">
      <Component Id="clipflowAgentExe" Guid="*">
        <File Source="$(var.AgentBinary)" KeyPath="yes" />
      </Component>
    </ComponentGroup>
  </Fragment>
</Wix>
"@ | Set-Content -Path $wxsPath -Encoding UTF8

$version = $env:VERSION
if (-not $version) { $version = '0.1.0' }
$candleArgs = @('-dVersion=' + $version, '-dAgentBinary=' + $binaryPath, $wxsPath)
$lightArgs = @('clipflow-agent.wixobj', '-o', (Join-Path $OutputDir 'clipflow-agent.msi'))

if (Get-Command candle.exe -ErrorAction SilentlyContinue) {
    Push-Location $wixDir
    Write-Host 'Compiling MSI using WiX Toolset'
    & candle.exe @candleArgs
    & light.exe @lightArgs
    Pop-Location
    Write-Host ('Installer created at ' + (Join-Path $OutputDir 'clipflow-agent.msi'))
} else {
    $zipPath = Join-Path $OutputDir 'clipflow-agent.zip'
    Write-Warning 'WiX Toolset not found. Creating portable zip archive instead.'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path $zipPath) { Remove-Item $zipPath }
    [System.IO.Compression.ZipFile]::CreateFromDirectory($binDir, $zipPath)
    Write-Host ('Created archive ' + $zipPath)
}

Write-Host 'TODO: Sign the executable and installer before distribution.'

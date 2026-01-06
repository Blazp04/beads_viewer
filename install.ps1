#Requires -Version 5.1
<#
.SYNOPSIS
    Install script for beads_viewer (bv) binary.
.DESCRIPTION
    Downloads and installs the bv binary from GitHub releases.
    Falls back to building from source if no pre-built binary is available.
#>

[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Configuration
$REPO_OWNER = "Dicklesworthstone"
$REPO_NAME = "beads_viewer"
$BIN_NAME = "bv"

# Track temporary directories for cleanup
$script:TMP_DIRS = @()

function Cleanup-TmpDirs {
    foreach ($dir in $script:TMP_DIRS) {
        if ($dir -and (Test-Path $dir)) {
            Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function New-TmpDir {
    $dir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $script:TMP_DIRS += $dir
    return $dir
}

# Register cleanup on script exit
$null = Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action { Cleanup-TmpDirs } -SupportEvent

function Get-DefaultInstallDir {
    if ($env:INSTALL_DIR) {
        return $env:INSTALL_DIR
    }

    # Prefer user's local bin directory
    $localBin = Join-Path $env:LOCALAPPDATA "Programs\bin"
    if (Test-Path $localBin) {
        return $localBin
    }

    # Check PATH for writable directories
    $pathEntries = $env:PATH -split ';'
    foreach ($dir in $pathEntries) {
        if ($dir -and (Test-Path $dir)) {
            try {
                $testFile = Join-Path $dir ".write_test_$(Get-Random)"
                [System.IO.File]::WriteAllText($testFile, "test")
                Remove-Item $testFile -Force
                return $dir
            } catch {
                continue
            }
        }
    }

    # Default to user's local programs bin
    return $localBin
}

$INSTALL_DIR = Get-DefaultInstallDir

function Write-Info {
    param([string]$Message)
    Write-Host "==> " -ForegroundColor Blue -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "==> " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Error2 {
    param([string]$Message)
    Write-Host "==> " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Write-Warn {
    param([string]$Message)
    Write-Host "==> " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Get-Platform {
    $os = "windows"
    $arch = if ([System.Environment]::Is64BitOperatingSystem) {
        $cpuArch = $env:PROCESSOR_ARCHITECTURE
        switch ($cpuArch) {
            "AMD64" { "amd64" }
            "ARM64" { "arm64" }
            default { "amd64" }
        }
    } else {
        Write-Error2 "32-bit systems are not supported"
        return $null
    }

    return "${os}_${arch}"
}

function Get-LatestRelease {
    $url = "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"
    
    try {
        $response = Invoke-RestMethod -Uri $url -Method Get -UseBasicParsing
        return $response
    } catch {
        Write-Warn "Failed to fetch latest release: $_"
        return $null
    }
}

function Get-File {
    param(
        [string]$Url,
        [string]$Destination
    )

    try {
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $Url -OutFile $Destination -UseBasicParsing
        return $true
    } catch {
        Write-Warn "Failed to download file: $_"
        return $false
    }
}

function Ensure-InstallDir {
    param([string]$Dir)

    if (Test-Path $Dir) {
        return $true
    }

    try {
        New-Item -ItemType Directory -Path $Dir -Force | Out-Null
        return $true
    } catch {
        Write-Error2 "Failed to create install directory: $Dir"
        return $false
    }
}

function Test-VersionGe {
    param(
        [string]$Version1,
        [string]$Version2
    )

    $v1Parts = $Version1 -split '\.' | ForEach-Object { [int]$_ }
    $v2Parts = $Version2 -split '\.' | ForEach-Object { [int]$_ }

    $maxLen = [Math]::Max($v1Parts.Count, $v2Parts.Count)

    for ($i = 0; $i -lt $maxLen; $i++) {
        $v1 = if ($i -lt $v1Parts.Count) { $v1Parts[$i] } else { 0 }
        $v2 = if ($i -lt $v2Parts.Count) { $v2Parts[$i] } else { 0 }

        if ($v1 -gt $v2) { return $true }
        if ($v1 -lt $v2) { return $false }
    }

    return $true
}

function Select-ReleaseAsset {
    param(
        [object]$ReleaseData,
        [string]$Platform
    )

    $ext = ".zip"
    $assets = $ReleaseData.assets

    # Prefer exact platform match with expected ext
    foreach ($asset in $assets) {
        $name = $asset.name
        if ($name -like "*$Platform*" -and $name.EndsWith($ext)) {
            return @{
                Version = $ReleaseData.tag_name
                Url = $asset.browser_download_url
                Name = $name
            }
        }
    }

    # Fallback: any asset that contains platform and correct ext
    $platformNorm = $Platform -replace '_', ''
    foreach ($asset in $assets) {
        $name = $asset.name
        $nameNorm = $name -replace '_', ''
        if ($nameNorm -like "*$platformNorm*" -and $name.EndsWith($ext)) {
            return @{
                Version = $ReleaseData.tag_name
                Url = $asset.browser_download_url
                Name = $name
            }
        }
    }

    return $null
}

function Ensure-Go {
    $minVersion = "1.21"

    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
        $goVersionOutput = & go version 2>$null
        if ($goVersionOutput -match 'go(\d+\.\d+(?:\.\d+)?)') {
            $goVersion = $Matches[1]
            if (Test-VersionGe $goVersion $minVersion) {
                return $goVersion
            }
            Write-Warn "Go $minVersion or later is required. Found: go$goVersion"
        }
    } else {
        Write-Warn "Go is not installed."
    }

    # Check if scoop is available
    $scoopCmd = Get-Command scoop -ErrorAction SilentlyContinue
    if ($scoopCmd) {
        $reply = Read-Host "Install/upgrade Go via Scoop now? [Y/n]"
        if ($reply -notmatch '^[Nn]') {
            try {
                & scoop install go
                $goVersionOutput = & go version 2>$null
                if ($goVersionOutput -match 'go(\d+\.\d+(?:\.\d+)?)') {
                    $goVersion = $Matches[1]
                    if (Test-VersionGe $goVersion $minVersion) {
                        Write-Success "Installed Go $goVersion via Scoop"
                        return $goVersion
                    }
                }
            } catch {
                Write-Warn "Scoop installation of Go failed."
            }
        }
    }

    # Check if choco is available
    $chocoCmd = Get-Command choco -ErrorAction SilentlyContinue
    if ($chocoCmd) {
        $reply = Read-Host "Install/upgrade Go via Chocolatey now? [Y/n]"
        if ($reply -notmatch '^[Nn]') {
            try {
                & choco install golang -y
                # Refresh PATH
                $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH", "User")
                $goVersionOutput = & go version 2>$null
                if ($goVersionOutput -match 'go(\d+\.\d+(?:\.\d+)?)') {
                    $goVersion = $Matches[1]
                    if (Test-VersionGe $goVersion $minVersion) {
                        Write-Success "Installed Go $goVersion via Chocolatey"
                        return $goVersion
                    }
                }
            } catch {
                Write-Warn "Chocolatey installation of Go failed."
            }
        }
    }

    # Check if winget is available
    $wingetCmd = Get-Command winget -ErrorAction SilentlyContinue
    if ($wingetCmd) {
        $reply = Read-Host "Install Go via winget now? [Y/n]"
        if ($reply -notmatch '^[Nn]') {
            try {
                & winget install GoLang.Go --silent
                # Refresh PATH
                $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH", "User")
                $goVersionOutput = & go version 2>$null
                if ($goVersionOutput -match 'go(\d+\.\d+(?:\.\d+)?)') {
                    $goVersion = $Matches[1]
                    if (Test-VersionGe $goVersion $minVersion) {
                        Write-Success "Installed Go $goVersion via winget"
                        return $goVersion
                    }
                }
            } catch {
                Write-Warn "winget installation of Go failed."
            }
        }
    }

    return $null
}

function Try-BinaryInstall {
    param([string]$Platform)

    Write-Info "Checking for pre-built binary..."

    $releaseData = Get-LatestRelease
    if (-not $releaseData) {
        return $false
    }

    $asset = Select-ReleaseAsset -ReleaseData $releaseData -Platform $Platform
    if (-not $asset) {
        Write-Warn "No pre-built binary found for $Platform"
        return $false
    }

    $version = $asset.Version
    if (-not $version) { $version = "unknown" }

    Write-Info "Latest release: $version"
    if ($asset.Name) {
        Write-Info "Selected asset: $($asset.Name)"
    }

    Write-Info "Downloading $($asset.Url)..."

    $tmpDir = New-TmpDir
    $archivePath = Join-Path $tmpDir "archive.zip"

    if (-not (Get-File -Url $asset.Url -Destination $archivePath)) {
        Write-Warn "Download failed"
        return $false
    }

    # Extract the binary
    Write-Info "Extracting..."

    try {
        Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force
    } catch {
        Write-Warn "Failed to extract archive: $_"
        return $false
    }

    # Find the binary in extracted contents
    $binaryPath = Get-ChildItem -Path $tmpDir -Recurse -Filter "$BIN_NAME.exe" -File | Select-Object -First 1

    if (-not $binaryPath) {
        $binaryPath = Get-ChildItem -Path $tmpDir -Recurse -Filter $BIN_NAME -File | Select-Object -First 1
    }

    if (-not $binaryPath) {
        Write-Warn "Binary not found in archive"
        return $false
    }

    # Install to destination
    if (-not (Ensure-InstallDir $INSTALL_DIR)) {
        return $false
    }

    $destPath = Join-Path $INSTALL_DIR "$BIN_NAME.exe"

    try {
        Copy-Item -Path $binaryPath.FullName -Destination $destPath -Force
    } catch {
        Write-Error2 "Failed to install binary: $_"
        return $false
    }

    Write-Success "Installed $BIN_NAME $version to $destPath"

    # Add to PATH if not already there
    Add-ToPath $INSTALL_DIR

    return $true
}

function Add-ToPath {
    param([string]$Dir)

    $currentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    if ($currentPath -notlike "*$Dir*") {
        Write-Info "Adding $Dir to user PATH..."
        $newPath = "$currentPath;$Dir"
        [System.Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        $env:PATH = "$env:PATH;$Dir"
        Write-Success "Added $Dir to PATH. You may need to restart your terminal."
    }
}

function Try-GoInstall {
    Write-Info "Attempting to build from source with go build..."

    $goVersion = Ensure-Go
    if (-not $goVersion) {
        Write-Error2 "Go 1.21 or later is required for building from source."
        return $false
    }

    Write-Info "Using Go $goVersion"

    $tmpDir = New-TmpDir
    $srcDir = Join-Path $tmpDir "src"
    $repoUrl = "https://github.com/$REPO_OWNER/$REPO_NAME.git"

    Write-Info "Fetching source..."

    $fetched = $false
    $gitCmd = Get-Command git -ErrorAction SilentlyContinue
    if ($gitCmd) {
        try {
            & git clone --depth 1 $repoUrl $srcDir 2>$null
            if (Test-Path $srcDir) {
                $fetched = $true
            }
        } catch {
            Write-Warn "git clone failed, attempting tarball download..."
        }
    }

    if (-not $fetched) {
        $tarballUrl = "https://codeload.github.com/$REPO_OWNER/$REPO_NAME/zip/refs/heads/main"
        $tarballPath = Join-Path $tmpDir "source.zip"

        if (-not (Get-File -Url $tarballUrl -Destination $tarballPath)) {
            Write-Error2 "Failed to download source tarball from GitHub."
            return $false
        }

        try {
            Expand-Archive -Path $tarballPath -DestinationPath $tmpDir -Force
        } catch {
            Write-Error2 "Failed to extract source archive: $_"
            return $false
        }

        $srcDir = Get-ChildItem -Path $tmpDir -Directory -Filter "$REPO_NAME-*" | Select-Object -First 1
        if (-not $srcDir) {
            Write-Error2 "Could not locate extracted source directory."
            return $false
        }
        $srcDir = $srcDir.FullName
    }

    Write-Info "Building $BIN_NAME from source..."
    $buildOutput = Join-Path $tmpDir "$BIN_NAME.exe"

    $env:GO111MODULE = "on"
    $env:CGO_ENABLED = "0"

    try {
        Push-Location $srcDir
        & go build -o $buildOutput "./cmd/$BIN_NAME"
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    } catch {
        Write-Error2 "Go build failed: $_"
        return $false
    } finally {
        Pop-Location
    }

    if (-not (Test-Path $buildOutput)) {
        Write-Error2 "Build output not found"
        return $false
    }

    if (-not (Ensure-InstallDir $INSTALL_DIR)) {
        return $false
    }

    $destPath = Join-Path $INSTALL_DIR "$BIN_NAME.exe"

    try {
        Move-Item -Path $buildOutput -Destination $destPath -Force
    } catch {
        Write-Error2 "Failed to install binary: $_"
        return $false
    }

    Write-Success "Built and installed $BIN_NAME from source to $destPath"

    # Add to PATH if not already there
    Add-ToPath $INSTALL_DIR

    return $true
}

function Main {
    try {
        Write-Info "Installing $BIN_NAME..."

        $platform = Get-Platform
        if (-not $platform) {
            Write-Warn "Could not detect platform, will try building from source"
            if (Try-GoInstall) {
                Write-Info "Run '$BIN_NAME' in any beads project to view issues."
            }
            return
        }

        Write-Info "Detected platform: $platform"

        # First, try to download pre-built binary
        if (Try-BinaryInstall $platform) {
            Write-Info "Run '$BIN_NAME' in any beads project to view issues."
            return
        }

        # Fall back to building from source
        Write-Info "Pre-built binary not available, falling back to source build..."
        if (Try-GoInstall) {
            Write-Info "Run '$BIN_NAME' in any beads project to view issues."
        }
    } finally {
        Cleanup-TmpDirs
    }
}

# Run main
Main

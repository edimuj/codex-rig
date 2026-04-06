$ErrorActionPreference = "Stop"

$repo = "edimuj/codex-rig"
$binary = "codex-rig"

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Error "Only 64-bit Windows is supported."
    exit 1
}

$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$version = $release.tag_name -replace '^v', ''
if (-not $version) {
    Write-Error "Failed to fetch latest version"
    exit 1
}

$archive = "${binary}_windows_${arch}.zip"
$url = "https://github.com/$repo/releases/download/v$version/$archive"

Write-Host "Downloading $binary v$version for windows/$arch..."

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmpDir | Out-Null

try {
    $zipPath = Join-Path $tmpDir $archive
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

    $installDir = Join-Path $env:LOCALAPPDATA "Programs\codex-rig"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir | Out-Null
    }

    Copy-Item (Join-Path $tmpDir "$binary.exe") -Destination $installDir -Force

    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -notlike "*$installDir*") {
        [Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
        $env:PATH = "$env:PATH;$installDir"
        Write-Host "Added $installDir to PATH (restart your terminal for it to take effect)."
    }

    Write-Host ""
    Write-Host "Installed $binary v$version to $installDir\$binary.exe"
    Write-Host ""
    Write-Host "Run '$binary --help' to get started."
}
finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}


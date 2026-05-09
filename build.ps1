param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$manifest = Join-Path $root "cmd\usb-suspend-watch\app.manifest"
$rsrc = Join-Path $root "cmd\usb-suspend-watch\rsrc_windows_amd64.syso"

function Invoke-Checked {
    $filePath = $args[0]
    $arguments = @()
    if ($args.Count -gt 1) {
        $arguments = $args[1..($args.Count - 1)]
    }

    & $filePath @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$filePath failed with exit code $LASTEXITCODE"
    }
}

Push-Location $root
try {
    New-Item -ItemType Directory -Force -Path "dist" | Out-Null
    Invoke-Checked go run github.com/akavel/rsrc@v0.10.2 -manifest $manifest -o $rsrc
    Invoke-Checked go test ./...

    $ldflags = "-s -w -H=windowsgui -X main.version=$Version"
    Invoke-Checked go build -trimpath -ldflags $ldflags -o "dist/usb-suspend-watch.exe" ./cmd/usb-suspend-watch

    $zip = "dist/usb-suspend-watch-x64.zip"
    if (Test-Path $zip) {
        Remove-Item $zip
    }
    Compress-Archive -Path "dist/usb-suspend-watch.exe", "README.md", "LICENSE", "CHANGELOG.md" -DestinationPath $zip

    Get-FileHash "dist/usb-suspend-watch.exe", $zip -Algorithm SHA256 |
        ForEach-Object { "$($_.Hash.ToLowerInvariant())  $(Split-Path $_.Path -Leaf)" } |
        Set-Content -Path "dist/SHA256SUMS.txt" -Encoding ASCII

    Write-Host "Built dist/usb-suspend-watch.exe"
    Write-Host "Built $zip"
    Write-Host "Built dist/SHA256SUMS.txt"
}
finally {
    if (Test-Path $rsrc) {
        Remove-Item $rsrc
    }
    Pop-Location
}

# Run the packaged release smoke on a native Windows host.
#
# This is the PowerShell counterpart of release-native-smoke.sh. It does not
# build or cross-run anything: the installer and desktop executable are read
# from the supplied release directory and launched on the current Windows
# host. The installer performs the authoritative manifest/checksum validation.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$ReleaseDir,
    [switch]$AllowAdhoc
)

$ErrorActionPreference = "Stop"

function Fail([string]$Message) {
    throw "release-native-smoke: $Message"
}

function Assert-RegularFile([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        Fail "$Label is missing: $Path"
    }
    $item = Get-Item -LiteralPath $Path -Force
    if ($item.LinkType -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        Fail "$Label must not be a symlink or reparse point: $Path"
    }
}

function Invoke-Checked([string]$File, [string[]]$Arguments) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        Fail "$File exited with code $LASTEXITCODE"
    }
}

$release = (Resolve-Path -LiteralPath $ReleaseDir).Path
if (-not (Test-Path -LiteralPath $release -PathType Container)) {
    Fail "release directory is not a directory: $release"
}
$manifestPath = Join-Path $release "metadata/release-manifest.json"
Assert-RegularFile $manifestPath "release manifest"
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
if ($manifest.schema_version -ne 1) { Fail "unsupported release manifest schema" }
if ([string]::IsNullOrWhiteSpace([string]$manifest.app.version)) { Fail "release manifest has no app version" }
$policy = $manifest.signing_policy
if ($null -eq $policy) { Fail "release manifest has no signing policy" }
if ($policy.windows.publisher_code_signing -ne $false) { Fail "Windows publisher code signing must be false" }
if ($policy.linux.publisher_code_signing -ne $false) { Fail "Linux publisher code signing must be false" }
if ($policy.macos.notarization -ne $false) { Fail "macOS notarization must be false" }

$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    "X64" { "amd64"; break }
    "Arm64" { "arm64"; break }
    default { Fail "unsupported native Windows architecture: $($_)" }
}
$target = "windows/$arch"
$targetKey = $target.Replace('/', '-')
$targets = @($manifest.targets | ForEach-Object { [string]$_ })
if (@($targets | Where-Object { $_.StartsWith("darwin/", [StringComparison]::Ordinal) }).Count -gt 0) {
    $macMode = [string]$policy.macos.mode
    if ($macMode -eq "ad-hoc/smoke") {
        if (-not $AllowAdhoc) { Fail "ad-hoc macOS output is smoke-only; pass -AllowAdhoc explicitly" }
        if ([string]$policy.macos.identity -ne "-") { Fail "ad-hoc macOS output must record identity '-'" }
    } elseif ($macMode -eq "project-self-signed") {
        $macIdentity = [string]$policy.macos.identity
        $unsafeIdentity = $false
        foreach ($character in $macIdentity.ToCharArray()) {
            if ([char]::IsControl($character)) {
                $unsafeIdentity = $true
                break
            }
        }
        if ([string]::IsNullOrWhiteSpace($macIdentity) -or $macIdentity -eq "-" -or $unsafeIdentity -or $macIdentity -match "(?i)developer id") {
            Fail "macOS community identity is missing, contains control characters, or uses Apple Developer ID"
        }
        $fingerprint = [string]$policy.macos.certificate_sha1
        if ($fingerprint -notmatch "^[A-Fa-f0-9]{40}$") {
            Fail "macOS community certificate_sha1 must be 40 hexadecimal characters"
        }
    } else {
        Fail "macOS release does not declare project-self-signed or explicit ad-hoc/smoke mode"
    }
}
if ($targets -notcontains $target) { Fail "release does not contain host target $target" }
$prefix = "artifacts/$targetKey/"
$entries = @($manifest.artifacts | Where-Object { ([string]$_.path).StartsWith($prefix, [StringComparison]::Ordinal) })
if ($entries.Count -eq 0) { Fail "release target has no artifacts: $target" }

# The installer is selected from the manifest rather than guessed from a
# directory listing, so an unlisted executable cannot become release evidence.
$installerEntry = $entries | Where-Object { [IO.Path]::GetFileName([string]$_.path) -eq "agent-remote-desktop-installer.exe" } | Select-Object -First 1
$appEntry = $entries | Where-Object { [IO.Path]::GetFileName([string]$_.path) -eq "agent-remote-desktop.exe" } | Select-Object -First 1
if ($null -eq $installerEntry) { Fail "Windows installer artifact is missing" }
if ($null -eq $appEntry) { Fail "Windows desktop executable is missing" }
$installerPath = Join-Path $release ([string]$installerEntry.path)
$appPath = Join-Path $release ([string]$appEntry.path)
Assert-RegularFile $installerPath "installer artifact"
Assert-RegularFile $appPath "desktop artifact"

# The release verifier is Bash/Python based. When present, run it as an
# additional policy check; the packaged installer still validates the release
# on Windows where Bash is unavailable.
$bash = Get-Command bash -ErrorAction SilentlyContinue
if ($null -ne $bash) {
    $verifyArgs = @((Join-Path $PSScriptRoot "verify-release.sh"), $release)
    if ($AllowAdhoc) { $verifyArgs += "--allow-adhoc" }
    Invoke-Checked $bash.Source $verifyArgs
}

# Install into a temporary user directory so the smoke run never mutates a
# system-wide Windows installation.
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("agent-remote-desktop-release-native-" + [guid]::NewGuid().ToString("N"))
$destination = Join-Path $tempRoot "installed"
try {
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
    Invoke-Checked $installerPath @(
        "--release", $release,
        "--destination", $destination,
        "--platform", "windows",
        "--arch", $arch
    )

    $statusPath = Join-Path $tempRoot "status.json"
    & $installerPath --status --destination $destination | Set-Content -Encoding utf8 -LiteralPath $statusPath
    if ($LASTEXITCODE -ne 0) { Fail "installer --status exited with code $LASTEXITCODE" }
    $status = Get-Content -Raw -LiteralPath $statusPath | ConvertFrom-Json
    if ($status.installed -ne $true -or $null -eq $status.current -or [string]$status.current.target -ne $target -or [string]$status.current.version -ne [string]$manifest.app.version) {
        Fail "unexpected installer status: $($statusPath)"
    }

    $relativeApp = ([string]$appEntry.path).Substring($prefix.Length)
    $installedApp = Join-Path $destination $relativeApp
    Assert-RegularFile $installedApp "installed desktop executable"
    $versionOutput = (& $installedApp --version | Out-String).Trim()
    $expectedVersion = "agent-remote-desktop $([string]$manifest.app.version)"
    if ($versionOutput -ne $expectedVersion) {
        Fail "installed binary version mismatch: got '$versionOutput', want '$expectedVersion'"
    }
    Write-Output $versionOutput
    Write-Output "release native smoke passed: $target ($destination)"
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -Recurse -Force -LiteralPath $tempRoot -ErrorAction SilentlyContinue
    }
}

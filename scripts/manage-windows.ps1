<#
.SYNOPSIS
    Manage native Windows GitHub Actions runners.

.DESCRIPTION
    Downloads, configures, starts, stops, and removes self-hosted GitHub Actions
    runners on Windows. Called from WSL2 via the runner CLI.

.PARAMETER Action
    One of: install, start, stop, remove

.PARAMETER ConfigFile
    Path to runners.yml (Windows path)

.PARAMETER RunnersDir
    Path to windows/runners/ directory (Windows path)

.PARAMETER Pat
    GitHub Personal Access Token
#>

param(
    [Parameter(Mandatory)]
    [ValidateSet("install", "start", "stop", "remove")]
    [string]$Action,

    [string]$ConfigFile,
    [string]$RunnersDir,
    [string]$Pat
)

$ErrorActionPreference = "Stop"

# Ensure runners directory exists
if ($RunnersDir -and -not (Test-Path $RunnersDir)) {
    New-Item -ItemType Directory -Path $RunnersDir -Force | Out-Null
}

function Get-WindowsRunners {
    param([string]$ConfigPath)

    # Use yq to extract windows runners as JSON
    $json = & yq -o=json '[.runners[] | select(.os == "windows")]' $ConfigPath 2>$null
    if (-not $json -or $json -eq "[]") {
        return @()
    }
    return ($json | ConvertFrom-Json)
}

function Get-LatestRunnerVersion {
    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/actions/runner/releases/latest" -Headers @{
        "Accept" = "application/vnd.github+json"
    }
    return $releases.tag_name -replace '^v', ''
}

function Get-RegistrationToken {
    param([string]$Repo, [string]$Token)

    $result = Invoke-RestMethod -Method Post `
        -Uri "https://api.github.com/repos/$Repo/actions/runners/registration-token" `
        -Headers @{
            "Authorization" = "Bearer $Token"
            "Accept"        = "application/vnd.github+json"
        }
    return $result.token
}

function Install-Runners {
    $runners = Get-WindowsRunners -ConfigPath $ConfigFile
    if ($runners.Count -eq 0) {
        Write-Host "No Windows runners defined in config."
        return
    }

    Write-Host "Fetching latest runner version..."
    $version = Get-LatestRunnerVersion
    $zipUrl = "https://github.com/actions/runner/releases/download/v${version}/actions-runner-win-x64-${version}.zip"
    $zipFile = Join-Path $env:TEMP "actions-runner-win-x64-${version}.zip"

    if (-not (Test-Path $zipFile)) {
        Write-Host "Downloading runner v${version}..."
        Invoke-WebRequest -Uri $zipUrl -OutFile $zipFile
    } else {
        Write-Host "Runner v${version} already downloaded."
    }

    foreach ($runner in $runners) {
        $count = if ($runner.count) { $runner.count } else { 1 }
        $labels = ($runner.labels -join ",")

        for ($i = 1; $i -le $count; $i++) {
            $name = "$($runner.name)-$i"
            $runnerDir = Join-Path $RunnersDir $name

            if (Test-Path (Join-Path $runnerDir ".runner")) {
                Write-Host "  $name: already configured, skipping."
                continue
            }

            Write-Host "  Installing $name..."

            # Create and extract
            if (-not (Test-Path $runnerDir)) {
                New-Item -ItemType Directory -Path $runnerDir -Force | Out-Null
            }
            Expand-Archive -Path $zipFile -DestinationPath $runnerDir -Force

            # Get registration token
            Write-Host "  Getting registration token for $($runner.repo)..."
            $regToken = Get-RegistrationToken -Repo $runner.repo -Token $Pat

            # Configure
            $configCmd = Join-Path $runnerDir "config.cmd"
            & $configCmd --unattended `
                --url "https://github.com/$($runner.repo)" `
                --token $regToken `
                --name $name `
                --labels $labels `
                --work "_work" `
                --replace

            Write-Host "  $name: configured."
        }
    }

    Write-Host "Done."
}

function Start-Runners {
    if (-not (Test-Path $RunnersDir)) {
        Write-Host "No runners installed. Run 'win-install' first."
        return
    }

    $dirs = Get-ChildItem -Path $RunnersDir -Directory
    foreach ($dir in $dirs) {
        $runCmd = Join-Path $dir.FullName "run.cmd"
        $pidFile = Join-Path $dir.FullName ".runner_pid"

        if (Test-Path $pidFile) {
            $pid = Get-Content $pidFile
            try {
                Get-Process -Id $pid -ErrorAction Stop | Out-Null
                Write-Host "  $($dir.Name): already running (PID $pid)"
                continue
            } catch {
                Remove-Item $pidFile -Force
            }
        }

        Write-Host "  Starting $($dir.Name)..."
        $proc = Start-Process -FilePath $runCmd -WorkingDirectory $dir.FullName -PassThru -WindowStyle Hidden
        $proc.Id | Out-File -FilePath $pidFile -NoNewline
        Write-Host "  $($dir.Name): started (PID $($proc.Id))"
    }
}

function Stop-Runners {
    if (-not (Test-Path $RunnersDir)) {
        Write-Host "No runners installed."
        return
    }

    $dirs = Get-ChildItem -Path $RunnersDir -Directory
    foreach ($dir in $dirs) {
        $pidFile = Join-Path $dir.FullName ".runner_pid"
        if (Test-Path $pidFile) {
            $pid = Get-Content $pidFile
            try {
                $proc = Get-Process -Id $pid -ErrorAction Stop
                Write-Host "  Stopping $($dir.Name) (PID $pid)..."
                Stop-Process -Id $pid -Force
                Remove-Item $pidFile -Force
                Write-Host "  $($dir.Name): stopped."
            } catch {
                Write-Host "  $($dir.Name): not running."
                Remove-Item $pidFile -Force
            }
        } else {
            Write-Host "  $($dir.Name): not running."
        }
    }
}

function Remove-Runners {
    $runners = Get-WindowsRunners -ConfigPath $ConfigFile
    if (-not (Test-Path $RunnersDir)) {
        Write-Host "No runners installed."
        return
    }

    # Stop first
    Stop-Runners

    $dirs = Get-ChildItem -Path $RunnersDir -Directory
    foreach ($dir in $dirs) {
        $configCmd = Join-Path $dir.FullName "config.cmd"

        if (Test-Path $configCmd) {
            # Find matching runner config to get the repo
            $name = $dir.Name
            $baseName = $name -replace '-\d+$', ''
            $runner = $runners | Where-Object { $_.name -eq $baseName }

            if ($runner) {
                Write-Host "  Deregistering $name from $($runner.repo)..."
                $regToken = Get-RegistrationToken -Repo $runner.repo -Token $Pat
                & $configCmd remove --token $regToken 2>$null
            }
        }

        Write-Host "  Removing $name directory..."
        Remove-Item -Path $dir.FullName -Recurse -Force
    }

    Write-Host "Done."
}

# Dispatch
switch ($Action) {
    "install" { Install-Runners }
    "start"   { Start-Runners }
    "stop"    { Stop-Runners }
    "remove"  { Remove-Runners }
}

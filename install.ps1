# Installs Moonwell on Windows: downloads moonwell.exe of this script's version from its GitHub release, checks it
# against the release's checksums, puts it in %LOCALAPPDATA%\moonwell\bin (the folder `moonwell setup` keeps yue in)
# and adds that folder to the user's PATH. Running it again upgrades.
#
#   irm https://github.com/mdlsvensson/moonwell/releases/latest/download/install.ps1 | iex

# In a script block: piped into iex, the script would otherwise leave its variables in the user's session.
& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'

  $version = '0.11.1'
  $base = "https://github.com/mdlsvensson/moonwell/releases/download/moonwell@$version"
  if ($env:MOONWELL_INSTALL_BASE) { $base = $env:MOONWELL_INSTALL_BASE }

  if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64' -and $env:PROCESSOR_ARCHITEW6432 -ne 'AMD64') {
    throw "Moonwell $version has no build for Windows on $($env:PROCESSOR_ARCHITECTURE). It is built for Linux and Windows on x86-64."
  }
  $asset = 'moonwell-windows-amd64.exe'

  $cache = Join-Path $env:LOCALAPPDATA 'moonwell'
  if ($env:MOONWELL_CACHE) { $cache = $env:MOONWELL_CACHE }
  $bin = Join-Path $cache 'bin'
  $target = Join-Path $bin 'moonwell.exe'

  # Windows PowerShell 5.1 does not offer TLS 1.2 by default, and GitHub accepts nothing older.
  [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

  $work = Join-Path ([IO.Path]::GetTempPath()) ('moonwell-install-' + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $work | Out-Null
  try {
    $download = Join-Path $work $asset
    $checksums = Join-Path $work 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $download
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $checksums

    $expected = $null
    foreach ($line in Get-Content $checksums) {
      $parts = $line -split '\s+', 2
      if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -eq $asset) { $expected = $parts[0] }
    }
    if (-not $expected) { throw "checksums.txt of Moonwell $version does not list $asset." }
    $actual = (Get-FileHash -Algorithm SHA256 -Path $download).Hash
    if ($actual -ne $expected) {
      throw "The download does not match its checksum (expected $expected, got $($actual.ToLower())). Nothing was installed."
    }

    New-Item -ItemType Directory -Force -Path $bin | Out-Null
    try {
      Move-Item -Force -Path $download -Destination $target
    } catch {
      throw "Writing $target failed: $($_.Exception.Message) Close every running moonwell (a terminal running moonwell dev), then run this again."
    }
  } finally {
    Remove-Item -Recurse -Force -Path $work -ErrorAction SilentlyContinue
  }
  Write-Host "Installed Moonwell $version to $target."

  if ($env:MOONWELL_INSTALL_NO_PATH -eq '1') { return }
  $entries = @([Environment]::GetEnvironmentVariable('Path', 'User') -split ';' | Where-Object { $_ })
  $wanted = $bin.TrimEnd('\')
  if (-not ($entries | Where-Object { $_.TrimEnd('\') -eq $wanted })) {
    [Environment]::SetEnvironmentVariable('Path', (($entries + $bin) -join ';'), 'User')
    Write-Host "Added $bin to your PATH. Open a new terminal to use moonwell everywhere."
  }
  if (-not (($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -eq $wanted })) {
    $env:Path = "$env:Path;$bin"
  }
}

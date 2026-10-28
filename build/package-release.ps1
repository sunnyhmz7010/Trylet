$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$releaseDirectory = Join-Path $projectRoot 'release\windows'
New-Item -ItemType Directory -Force -Path $releaseDirectory | Out-Null

$executable = Join-Path $releaseDirectory 'Trylet.exe'
Copy-Item -LiteralPath (Join-Path $projectRoot 'bin\Trylet.exe') -Destination $executable -Force
$portableFiles = @(
    $executable,
    (Join-Path $projectRoot '使用说明.md'),
    (Join-Path $projectRoot 'LICENSE'),
    (Join-Path $projectRoot 'THIRD_PARTY_NOTICES.txt')
)
Compress-Archive -LiteralPath $portableFiles -DestinationPath (Join-Path $releaseDirectory 'Trylet-portable.zip') -Force

$sourceArchive = Join-Path $releaseDirectory 'Trylet-source.zip'
git -C $projectRoot archive --format=zip --prefix=Trylet/ --output=$sourceArchive HEAD
if ($LASTEXITCODE -ne 0) { throw '源码打包失败' }

$checksums = foreach ($fileName in @('Trylet.exe', 'Trylet-portable.zip', 'Trylet-source.zip')) {
    $hash = (Get-FileHash -LiteralPath (Join-Path $releaseDirectory $fileName) -Algorithm SHA256).Hash.ToLowerInvariant()
    $hash + '  ' + $fileName
}
[IO.File]::WriteAllText((Join-Path $releaseDirectory 'Trylet-SHA256SUMS.txt'), ($checksums -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
Write-Host ('已打包：' + $releaseDirectory)

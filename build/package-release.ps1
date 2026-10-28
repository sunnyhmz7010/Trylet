$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$releaseDirectory = Join-Path $projectRoot 'release\windows'
New-Item -ItemType Directory -Force -Path $releaseDirectory | Out-Null

$sourceExecutable = Join-Path $projectRoot 'bin\Trylet.exe'
# 从 PE 头读取实际架构，避免包名与二进制内容不一致。
$executableBytes = [IO.File]::ReadAllBytes($sourceExecutable)
$peOffset = [BitConverter]::ToInt32($executableBytes, 0x3c)
$machine = [BitConverter]::ToUInt16($executableBytes, $peOffset + 4)
$architecture = switch ($machine) {
    0x8664 { 'amd64' }
    0xaa64 { 'arm64' }
    default { throw ('不支持的 EXE 架构：0x{0:X4}' -f $machine) }
}
$packageName = 'Trylet-windows-' + $architecture
$executable = Join-Path $releaseDirectory ($packageName + '.exe')
Copy-Item -LiteralPath $sourceExecutable -Destination $executable -Force
$portableFiles = @(
    $sourceExecutable,
    (Join-Path $projectRoot '使用说明.md'),
    (Join-Path $projectRoot 'LICENSE'),
    (Join-Path $projectRoot 'THIRD_PARTY_NOTICES.txt')
)
Compress-Archive -LiteralPath $portableFiles -DestinationPath (Join-Path $releaseDirectory ($packageName + '-portable.zip')) -Force

$fileNames = @(($packageName + '.exe'), ($packageName + '-portable.zip'))
if ($architecture -eq 'amd64') {
    $sourceArchive = Join-Path $releaseDirectory 'Trylet-source.zip'
    git -C $projectRoot archive --format=zip --prefix=Trylet/ --output=$sourceArchive HEAD
    if ($LASTEXITCODE -ne 0) { throw '源码打包失败' }
    $fileNames += 'Trylet-source.zip'
}

$checksums = foreach ($fileName in $fileNames) {
    $hash = (Get-FileHash -LiteralPath (Join-Path $releaseDirectory $fileName) -Algorithm SHA256).Hash.ToLowerInvariant()
    $hash + '  ' + $fileName
}
[IO.File]::WriteAllText((Join-Path $releaseDirectory ($packageName + '-SHA256SUMS.txt')), ($checksums -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
Write-Host ('已打包：' + $releaseDirectory)

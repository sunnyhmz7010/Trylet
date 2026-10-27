$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$glyph = [xml][IO.File]::ReadAllText((Join-Path $PSScriptRoot '..\frontend\src\brand.svg'))
$polygon = $glyph.svg.polygon
$markPoints = @($polygon.points.Split(' ', [StringSplitOptions]::RemoveEmptyEntries) | ForEach-Object {
    $xy = $_.Split(',')
    [Drawing.PointF]::new([single]$xy[0], [single]$xy[1])
})
$svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img"><title>Trylet</title><rect width="32" height="32" rx="7.36" fill="#4f46e5"/><g transform="translate(4 4) scale(.75)">' + $polygon.OuterXml.Replace('currentColor', '#ffffff') + '</g></svg>'
[IO.File]::WriteAllText((Join-Path $PSScriptRoot 'icon.svg'), $svg + "`n", [Text.UTF8Encoding]::new($false))
$sizes = @(16, 20, 24, 32, 40, 48, 64, 128, 256)
$frames = [Collections.Generic.List[byte[]]]::new()
foreach ($size in $sizes) {
    $side = $size * 4
    $large = [Drawing.Bitmap]::new($side, $side)
    $graphics = [Drawing.Graphics]::FromImage($large)
    $tile = [Drawing.Drawing2D.GraphicsPath]::new()
    $purple = [Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(79, 70, 229))
    $white = [Drawing.SolidBrush]::new([Drawing.Color]::White)
    $bitmap = [Drawing.Bitmap]::new($size, $size)
    $small = [Drawing.Graphics]::FromImage($bitmap)
    $stream = [IO.MemoryStream]::new()
    try {
        $graphics.Clear([Drawing.Color]::Transparent)
        $graphics.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $diameter = [single]($side * .46)
        $edge = [single]($side - 1)
        $tile.AddArc(0, 0, $diameter, $diameter, 180, 90)
        $tile.AddArc(($edge - $diameter), 0, $diameter, $diameter, 270, 90)
        $tile.AddArc(($edge - $diameter), ($edge - $diameter), $diameter, $diameter, 0, 90)
        $tile.AddArc(0, ($edge - $diameter), $diameter, $diameter, 90, 90)
        $tile.CloseFigure()
        $graphics.FillPath($purple, $tile)
        $points = [Drawing.PointF[]]@($markPoints | ForEach-Object {
            [Drawing.PointF]::new([single](($_.X * .75 + 4) * $side / 32), [single](($_.Y * .75 + 4) * $side / 32))
        })
        $graphics.FillPolygon($white, $points)
        $small.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $small.CompositingMode = [Drawing.Drawing2D.CompositingMode]::SourceCopy
        $small.DrawImage($large, [Drawing.Rectangle]::new(0, 0, $size, $size), [Drawing.Rectangle]::new(0, 0, $side, $side), [Drawing.GraphicsUnit]::Pixel)
        $bitmap.Save($stream, [Drawing.Imaging.ImageFormat]::Png)
        $frames.Add($stream.ToArray())
        if ($size -eq 256) { $bitmap.Save((Join-Path $PSScriptRoot 'icon.png'), [Drawing.Imaging.ImageFormat]::Png) }
    } finally {
        $stream.Dispose(); $small.Dispose(); $bitmap.Dispose(); $white.Dispose(); $purple.Dispose(); $tile.Dispose(); $graphics.Dispose(); $large.Dispose()
    }
}
$writer = [IO.BinaryWriter]::new([IO.File]::Create((Join-Path $PSScriptRoot 'icon.ico')))
try {
    $writer.Write([uint16]0); $writer.Write([uint16]1); $writer.Write([uint16]$sizes.Count)
    $offset = 6 + 16 * $sizes.Count
    for ($index = 0; $index -lt $sizes.Count; $index++) {
        $dimension = if ($sizes[$index] -eq 256) { 0 } else { $sizes[$index] }
        $writer.Write([byte]$dimension); $writer.Write([byte]$dimension); $writer.Write([byte]0); $writer.Write([byte]0)
        $writer.Write([uint16]1); $writer.Write([uint16]32)
        $writer.Write([uint32]$frames[$index].Length); $writer.Write([uint32]$offset)
        $offset += $frames[$index].Length
    }
    foreach ($frame in $frames) { $writer.Write($frame) }
} finally { $writer.Dispose() }

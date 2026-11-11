package build

import _ "embed"

// Icon 是应用窗口和系统托盘共用的图标。
//
//go:embed icon.png
var Icon []byte

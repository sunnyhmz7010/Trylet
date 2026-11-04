package frontend

import "embed"

// Assets 包含构建后的前端资源，由桌面入口传入。
//go:embed all:dist
var Assets embed.FS

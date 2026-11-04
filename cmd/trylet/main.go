package main

import (
	"trylet/build"
	"trylet/frontend"
	"trylet/internal/desktop"
)

func main() {
	desktop.Run(frontend.Assets, build.Icon)
}

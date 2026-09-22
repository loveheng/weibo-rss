// Package weiborss 将前端静态资源嵌入二进制，
// 使服务可以以单文件形态运行（scratch 镜像 / 裸二进制）。
package weiborss

import "embed"

// Public 为 public/ 目录下的静态资源（首页转换工具、favicon 等）。
//
//go:embed all:public
var Public embed.FS

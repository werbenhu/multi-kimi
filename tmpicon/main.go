//go:build ignore

// 一次性图标生成器：go run tmpicon/main.go
// 生成 build/appicon.png (256x256)、build/windows/icon.ico (PNG 内嵌 ICO)
// 以及 frontend/public/logo.png。
// 图案：紫蓝对角渐变圆角底 + 白色 "MK" 字母（几何笔画，4x 超采样抗锯齿）。
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	outSize   = 256
	ssFactor  = 4 // 超采样倍数
	renderSz  = outSize * ssFactor
	cornerRad = 56 * ssFactor // 圆角半径
)

type pt struct{ x, y float64 }

// seg 一条笔画线段（相对单个字母框的 0..1 坐标）。
type seg struct{ a, b pt }

var (
	letterM = []seg{
		{pt{0, 1}, pt{0, 0}},
		{pt{0, 0}, pt{0.5, 0.62}},
		{pt{0.5, 0.62}, pt{1, 0}},
		{pt{1, 0}, pt{1, 1}},
	}
	letterK = []seg{
		{pt{0, 1}, pt{0, 0}},
		{pt{0.02, 0.52}, pt{1, 0}},
		{pt{0.18, 0.42}, pt{1, 1}},
	}
)

func main() {
	img := image.NewRGBA(image.Rect(0, 0, renderSz, renderSz))

	c1 := color.RGBA{R: 0x4f, G: 0x7c, B: 0xff, A: 0xff} // 蓝
	c2 := color.RGBA{R: 0x7a, G: 0x4f, B: 0xff, A: 0xff} // 紫
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	// 字母布局（渲染坐标）：字高 52%，M 宽 29%，K 宽 26%，间距 13%，整体居中
	letterH := 0.52 * renderSz
	mW := 0.29 * renderSz
	kW := 0.26 * renderSz
	gap := 0.13 * renderSz
	totalW := mW + gap + kW
	originX := (renderSz - totalW) / 2
	originY := (renderSz - letterH) / 2
	strokeW := 0.155 * letterH // 笔画粗细

	// M 的坐标系按宽度 mW 缩放；K 按 kW。高度都用 letterH。
	mBox := box{originX, originY, mW, letterH}
	kBox := box{originX + mW + gap, originY, kW, letterH}

	half := strokeW / 2
	aa := float64(ssFactor) * 1.2 // 边缘柔化宽度（渲染像素）

	for y := 0; y < renderSz; y++ {
		for x := 0; x < renderSz; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5

			// 圆角矩形底（带边缘抗锯齿）
			d := roundedRectDist(fx, fy, renderSz, cornerRad)
			if d > aa {
				continue
			}

			// 对角渐变
			t := (fx + fy) / (2*renderSz - 2)
			px := color.RGBA{
				R: lerp(c1.R, c2.R, t),
				G: lerp(c1.G, c2.G, t),
				B: lerp(c1.B, c2.B, t),
				A: 0xff,
			}

			// 字母笔画：到最近线段的距离决定覆盖度
			cover := 0.0
			for _, s := range letterM {
				cover = math.Max(cover, strokeCover(mBox.mapPt(s.a), mBox.mapPt(s.b), fx, fy, half, aa))
			}
			for _, s := range letterK {
				cover = math.Max(cover, strokeCover(kBox.mapPt(s.a), kBox.mapPt(s.b), fx, fy, half, aa))
			}
			if cover > 0 {
				px = blend(px, white, cover)
			}
			img.SetRGBA(x, y, px)
		}
	}

	small := downscale(img, ssFactor)

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, small); err != nil {
		panic(err)
	}
	os.MkdirAll("build/windows", 0o755)
	must(os.WriteFile(filepath.Join("build", "appicon.png"), pngBuf.Bytes(), 0o644))
	must(os.WriteFile(filepath.Join("frontend", "public", "logo.png"), pngBuf.Bytes(), 0o644))

	// ICO: ICONDIR + ICONDIRENTRY + PNG payload（256x256 PNG 压缩条目）
	ico := &bytes.Buffer{}
	ico.Write([]byte{0, 0, 1, 0, 1, 0}) // reserved, type=icon, count=1
	entry := make([]byte, 12)
	entry[0], entry[1] = 0, 0 // 256 → 0
	entry[2] = 0              // colors
	entry[4] = 1              // planes
	binary.LittleEndian.PutUint16(entry[6:], 32)
	binary.LittleEndian.PutUint32(entry[8:], uint32(pngBuf.Len()))
	ico.Write(entry)
	var off [4]byte
	binary.LittleEndian.PutUint32(off[:], 22)
	ico.Write(off[:])
	ico.Write(pngBuf.Bytes())
	must(os.WriteFile(filepath.Join("build", "windows", "icon.ico"), ico.Bytes(), 0o644))
	println("icons generated")
}

type box struct{ x, y, w, h float64 }

func (b box) mapPt(p pt) pt { return pt{b.x + p.x*b.w, b.y + p.y*b.h} }

func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + t*(float64(b)-float64(a)))
}

// roundedRectDist 点到圆角矩形边缘的距离（内部为负）。
func roundedRectDist(x, y, size, radius float64) float64 {
	cx, cy := size/2, size/2
	qx := math.Abs(x-cx) - (size/2 - radius)
	qy := math.Abs(y-cy) - (size/2 - radius)
	ax, ay := math.Max(qx, 0), math.Max(qy, 0)
	return math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - radius
}

// strokeCover 点到线段构成的圆头笔画的覆盖度（0..1，带 aa 柔化）。
func strokeCover(a, b pt, x, y, half, aa float64) float64 {
	d := distToSeg(pt{x, y}, a, b) - half
	switch {
	case d <= -aa:
		return 1
	case d >= aa:
		return 0
	default:
		return 0.5 - d/(2*aa)
	}
}

func distToSeg(p, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	l2 := dx*dx + dy*dy
	t := ((p.x-a.x)*dx + (p.y-a.y)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

func blend(base, top color.RGBA, cover float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(base.R)*(1-cover) + float64(top.R)*cover),
		G: uint8(float64(base.G)*(1-cover) + float64(top.G)*cover),
		B: uint8(float64(base.B)*(1-cover) + float64(top.B)*cover),
		A: 0xff,
	}
}

// downscale 简单盒式平均降采样。
func downscale(img *image.RGBA, factor int) *image.RGBA {
	n := img.Bounds().Dx() / factor
	out := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var r, g, b, a2 int
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					c := img.RGBAAt(x*factor+dx, y*factor+dy)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a2 += int(c.A)
				}
			}
			n2 := factor * factor
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n2), G: uint8(g / n2), B: uint8(b / n2), A: uint8(a2 / n2),
			})
		}
	}
	return out
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

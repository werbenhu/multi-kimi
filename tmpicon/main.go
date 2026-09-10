//go:build ignore

// Regenerate PNG and multi-resolution ICO assets: go run tmpicon/main.go
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

type point struct{ x, y float64 }

// Two opposing folded arrows represent switching between account slots.
var forward = []point{{58, 114}, {58, 78}, {150, 78}, {150, 54}, {200, 96}, {150, 138}, {150, 114}}
var backward = []point{{198, 142}, {198, 178}, {106, 178}, {106, 202}, {56, 160}, {106, 118}, {106, 142}}

func inside(x, y float64, polygon []point) bool {
	hit := false
	j := len(polygon) - 1
	for i, p := range polygon {
		q := polygon[j]
		if (p.y > y) != (q.y > y) && x < (q.x-p.x)*(y-p.y)/(q.y-p.y)+p.x {
			hit = !hit
		}
		j = i
	}
	return hit
}

// Render every size independently with 4x supersampling and transparent corners.
func render(size int) []byte {
	const samples = 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, count int
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := (float64(x) + (float64(sx)+0.5)/samples) * 256 / float64(size)
					fy := (float64(y) + (float64(sy)+0.5)/samples) * 256 / float64(size)
					qx, qy := math.Abs(fx-128)-68, math.Abs(fy-128)-68
					if math.Hypot(math.Max(qx, 0), math.Max(qy, 0))+math.Min(math.Max(qx, qy), 0) > 56 {
						continue
					}
					t := (fx + fy) / 512
					c := color.NRGBA{uint8(57 - 32*t), uint8(60 - 34*t), uint8(119 - 57*t), 255}
					if inside(fx, fy, forward) {
						c = color.NRGBA{247, 249, 255, 255}
					}
					if inside(fx, fy, backward) {
						c = color.NRGBA{112, 235, 208, 255}
					}
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					count++
				}
			}
			if count > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(count * 255 / (samples * samples))})
			}
		}
	}
	var buf bytes.Buffer
	must(png.Encode(&buf, img))
	return buf.Bytes()
}

func main() {
	must(os.MkdirAll("build/windows", 0755))
	must(os.MkdirAll("frontend/public", 0755))
	must(os.WriteFile("build/appicon.png", render(1024), 0644))
	must(os.WriteFile("frontend/public/logo.png", render(256), 0644))
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	header := make([]byte, 6+16*len(sizes))
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], uint16(len(sizes)))
	var payload bytes.Buffer
	for i, size := range sizes {
		data := render(size)
		entry := header[6+i*16 : 6+(i+1)*16]
		entry[0], entry[1] = byte(size%256), byte(size%256)
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(data)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(len(header)+payload.Len()))
		payload.Write(data)
	}
	must(os.WriteFile("build/windows/icon.ico", append(header, payload.Bytes()...), 0644))
	println("Generated 1024px app icon, 256px logo, and 9 Windows icon sizes (16-256px).")
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Job struct {
	SrcPath   string
	OutDir    string
	Width     int
	ThumbSize int
	Watermark string
}

type JobResult struct {
	SrcPath string
	Bytes   int64
	Elapsed time.Duration
	Err     error
}

// 1. Масштабирование (Nearest-Neighbor)
func scaleImg(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xr, yr := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + int(float64(x)*xr)
			sy := b.Min.Y + int(float64(y)*yr)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// 2. Оттенки серого (Grayscale)
func grayImg(src image.Image) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, color.GrayModel.Convert(src.At(x, y)))
		}
	}
	return dst
}

// 3. Квадратная миниатюра по центру (Thumbnail)
func thumbImg(src image.Image, size int) *image.RGBA {
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	crop := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(crop, crop.Bounds(), src, image.Point{x0, y0}, draw.Src)
	return scaleImg(crop, size, size)
}

// 4. Фильтр Гаусса 3x3 (Дополнительное задание)
func gaussianBlur(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	kernel := [3][3]uint32{{1, 2, 1}, {2, 4, 2}, {1, 2, 1}}

	for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
		for x := b.Min.X + 1; x < b.Max.X-1; x++ {
			var rSum, gSum, bSum uint32
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					r, g, bl, _ := src.At(x+kx, y+ky).RGBA()
					w := kernel[ky+1][kx+1]
					rSum += (r >> 8) * w
					gSum += (g >> 8) * w
					bSum += (bl >> 8) * w
				}
			}
			dst.Set(x, y, color.RGBA{uint8(rSum / 16), uint8(gSum / 16), uint8(bSum / 16), 255})
		}
	}
	return dst
}

// 5. Наложение текстового водяного знака (Обязательное задание)
func addWatermark(src image.Image, text string) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 220}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(b.Min.X+15, b.Max.Y-15),
	}
	d.DrawString(text)
	return dst
}

func writeImg(img image.Image, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(path), ".png") {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	}
	if err != nil {
		return 0, err
	}
	info, _ := f.Stat()
	return info.Size(), nil
}

func handleJob(j Job) JobResult {
	t0 := time.Now()
	inInfo, err := os.Stat(j.SrcPath)
	if err != nil {
		return JobResult{SrcPath: j.SrcPath, Err: err}
	}

	f, err := os.Open(j.SrcPath)
	if err != nil {
		return JobResult{SrcPath: j.SrcPath, Err: err}
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return JobResult{SrcPath: j.SrcPath, Err: err}
	}

	base := strings.TrimSuffix(filepath.Base(j.SrcPath), filepath.Ext(j.SrcPath))
	b := img.Bounds()
	newH := int(float64(b.Dy()) * float64(j.Width) / float64(b.Dx()))

	outputs := map[string]image.Image{
		"_resize.jpg":    scaleImg(img, j.Width, newH),
		"_gray.jpg":      grayImg(img),
		"_thumb.jpg":     thumbImg(img, j.ThumbSize),
		"_blur.jpg":      gaussianBlur(img),
		"_watermark.jpg": addWatermark(img, j.Watermark),
	}

	totalBytes := inInfo.Size()
	for suffix, outImg := range outputs {
		sz, err := writeImg(outImg, filepath.Join(j.OutDir, base+suffix))
		if err != nil {
			return JobResult{SrcPath: j.SrcPath, Err: err}
		}
		totalBytes += sz
	}

	return JobResult{SrcPath: j.SrcPath, Bytes: totalBytes, Elapsed: time.Since(t0)}
}

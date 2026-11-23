package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

func generateSamples(dir string, n int) {
	_ = os.MkdirAll(dir, 0755)
	for i := 1; i <= n; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 640, 480))
		for y := 0; y < 480; y++ {
			for x := 0; x < 640; x++ {
				img.Set(x, y, color.RGBA{uint8((x + i*30) % 255), uint8((y + i*20) % 255), 140, 255})
			}
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("sample_%02d.jpg", i)))
		if err == nil {
			_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
			f.Close()
		}
	}
}

func findImages(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var list []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".bmp") {
			list = append(list, filepath.Join(dir, e.Name()))
		}
	}
	return list
}

func main() {
	inDir := flag.String("dir", "test_images", "Входная папка")
	outDir := flag.String("out", "output_images", "Выходная папка")
	workers := flag.Int("workers", 2, "Начальное число воркеров")
	maxWorkers := flag.Int("max-workers", runtime.NumCPU()*2, "Максимум воркеров (адаптивно)")
	gen := flag.Int("gen", 12, "Сгенерировать N тестовых картинок")
	bench := flag.Bool("bench", false, "Режим сравнения скорости")
	wmText := flag.String("wm", "PR-11 SYSINFO GO", "Текст водяного знака")
	flag.Parse()

	if *gen > 0 {
		generateSamples(*inDir, *gen)
	}
	_ = os.MkdirAll(*outDir, 0755)

	files := findImages(*inDir)
	if len(files) == 0 {
		fmt.Println("Изображения не найдены!")
		return
	}

	var jobs []Job
	for _, path := range files {
		jobs = append(jobs, Job{SrcPath: path, OutDir: *outDir, Width: 320, ThumbSize: 120, Watermark: *wmText})
	}

	if *bench {
		runBenchmark(jobs, []int{1, 2, 4, runtime.NumCPU()})
		return
	}

	fmt.Printf("🚀 Обработка %d изображений (Старт воркеров: %d, Макс: %d)...\n", len(jobs), *workers, *maxWorkers)
	pool := NewAdaptivePool(*workers, *maxWorkers, 4)
	wallStart := time.Now()

	go func() {
		for _, j := range jobs {
			pool.Submit(j)
		}
		pool.CloseAndWait()
	}()

	var okCount, errCount int
	var totalBytes int64
	var minTime, maxTime, sumTime time.Duration

	for res := range pool.results {
		if res.Err != nil {
			errCount++
			fmt.Printf("  ❌ Ошибка %s: %v\n", filepath.Base(res.SrcPath), res.Err)
			continue
		}
		okCount++
		totalBytes += res.Bytes
		sumTime += res.Elapsed
		if minTime == 0 || res.Elapsed < minTime {
			minTime = res.Elapsed
		}
		if res.Elapsed > maxTime {
			maxTime = res.Elapsed
		}
		fmt.Printf("  ✅ %s обработан за %v\n", filepath.Base(res.SrcPath), res.Elapsed.Round(time.Millisecond))
	}

	avgTime := time.Duration(0)
	if okCount > 0 {
		avgTime = sumTime / time.Duration(okCount)
	}

	fmt.Println("\n=== Подробная статистика ===")
	fmt.Printf("Успешно / Ошибок:     %d / %d\n", okCount, errCount)
	fmt.Printf("Активных воркеров:    %d (адаптивное масштабирование)\n", atomic.LoadInt32(&pool.active))
	fmt.Printf("Время на 1 файл:      Мин: %v | Макс: %v | Среднее: %v\n",
		minTime.Round(time.Millisecond), maxTime.Round(time.Millisecond), avgTime.Round(time.Millisecond))
	fmt.Printf("Объём данных (вх+вых): %.2f МБ\n", float64(totalBytes)/(1024*1024))
	fmt.Printf("Общее время работы:   %v\n", time.Since(wallStart).Round(time.Millisecond))
}

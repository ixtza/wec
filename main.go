package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"ixtza/ajk/wec/algo/lfu"
	"ixtza/ajk/wec/algo/lirs"
	"ixtza/ajk/wec/algo/lru"
	"ixtza/ajk/wec/algo/wec_v5"
	"ixtza/ajk/wec/simulator"
)

func main() {
	var (
		traces    []simulator.Trace = make([]simulator.Trace, 0)
		simulator simulator.Simulator
		timeStart time.Time
		out       *os.File
		fs        os.FileInfo
		filePath  string
		outPath   string
		algorithm string
		err       error
		cacheList []int
	)

	algo := flag.String("algo", "", "algorithm\n(LIRS|LRU|LFU|WEC)")
	pathfile := flag.String("filepath", "", "lokasi file trace dalam direktori")
	updatingPeriod := flag.Int("wec-update-periode", 0, "periode pembaruan cache")
	quitThresholdType := flag.String("wec-qt-type", "", "tipe konfigurasi batas umur cache\n(cube-root|square-root|cubic|quadratic|linear)")
	ramPercentage := flag.Float64("wec-ram-percentage", 0, "rasio ram terhadap cache")
	capacitySizeRatio := flag.Float64("wec-capacity-ratio", 0, "rasio cache terhadap memori")
	wecDataThreshold := flag.Float64("wec-threshold", 0, "batas rasio pengambilan kandidat cache")
	baseDir := flag.String("basedir", "", "lokasi dasar penyimpanan keluaran")

	flag.Parse()

	if len(os.Args) < 4 {
		fmt.Println("program [algorithm(LIRS|LRU|LFU|WEC)] [file] [trace size]...")
		os.Exit(1)
	}

	algorithm = *algo
	filePath = *pathfile
	baseDirectory := *baseDir
	flag.Parse()
	capacitySize := flag.Args()

	if fs, err = os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("%v does not exists\n", filePath)
		os.Exit(1)
	}

	cacheList, err = validateTraceSize(capacitySize)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	traces, err = readFile(filePath)
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}

	fileName := strings.Split(fs.Name(), ".")[0]
	basePath := fmt.Sprintf("./output/%v", algorithm)
	if baseDirectory != "" {
		basePath = fmt.Sprintf("%v/%v", basePath, basePath)
	}

	if err := os.MkdirAll(basePath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	outPath = fmt.Sprintf("%v/%v_%v_%v.txt", basePath, time.Now().Unix(), algorithm, fileName)
	if strings.ToLower(algorithm) == "wecv5" {
		outPath = fmt.Sprintf("%v/%v_%v_%v_%v_%v_%v_%v.txt", basePath, algorithm, fileName, *capacitySizeRatio, *ramPercentage, *quitThresholdType, *updatingPeriod, time.Now().Unix())
	}

	out, err = os.Create(outPath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer out.Close()

	if strings.ToLower(algorithm) == "wecv5" {
		for _, cache := range cacheList {
			simulator := wec_v5.New(
				cache,
				*updatingPeriod,
				*quitThresholdType,
				float32(*ramPercentage),
				float32(*capacitySizeRatio),
				float32(*wecDataThreshold),
			)
			timeStart = time.Now()

			for _, trace := range traces {
				err = simulator.Get(trace)
				if err != nil {
					log.Fatal(err.Error())
				}
			}

			simulator.PrintToFile(out, timeStart)
		}
	} else {
		for _, cache := range cacheList {
			switch strings.ToLower(algorithm) {
			case "lirs":
				simulator = lirs.NewLIRS(cache, 1)
			case "lru":
				simulator = lru.NewLRU(cache)
			case "lfu":
				simulator = lfu.NewLFU(cache)
			default:
				log.Fatal("algorithm not supported")
			}

			timeStart = time.Now()

			for _, trace := range traces {
				err = simulator.Get(trace)
				if err != nil {
					log.Fatal(err.Error())
				}
			}

			simulator.PrintToFile(out, timeStart)
		}
	}

	fmt.Println(algorithm)
	fmt.Println(outPath)
	fmt.Println("Done")
}

func validateTraceSize(tracesize []string) (sizeList []int, err error) {
	var (
		cacheList []int
		cache     int
	)

	for _, size := range tracesize {
		cache, err = strconv.Atoi(size)
		if err != nil {
			return sizeList, err
		}
		cacheList = append(cacheList, cache)
	}
	return cacheList, nil
}

func readFile(filePath string) (traces []simulator.Trace, err error) {
	var (
		file    *os.File
		scanner *bufio.Scanner
		row     []string
		address int
	)
	file, err = os.Open(filePath)
	if err != nil {
		return traces, err
	}
	defer file.Close()

	scanner = bufio.NewScanner(file)

	for scanner.Scan() {
		row = strings.Split(scanner.Text(), ",")
		address, err = strconv.Atoi(row[0])
		if err != nil {
			return traces, err
		}
		traces = append(traces,
			simulator.Trace{
				Addr: address,
				Op:   row[1],
			},
		)
	}

	return traces, nil
}

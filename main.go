package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"ixtza/ajk/wec/algo/wec_v2"
	"ixtza/ajk/wec/algo/wec"
	"ixtza/ajk/wec/algo/lfu"
	"ixtza/ajk/wec/algo/lirs"
	"ixtza/ajk/wec/algo/lru"
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
		cacheListWEC [][]int
	)

	if len(os.Args) < 4 {
		fmt.Println("program [algorithm(LIRS|LRU|LFU|WEC)] [file] [trace size]...")
		os.Exit(1)
	}

	algorithm = os.Args[1]

	filePath = os.Args[2]
	if fs, err = os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("%v does not exists\n", filePath)
		os.Exit(1)
	}

	if (strings.ToLower(algorithm) == "wec") {
		// CP ratio
		cache, err := strconv.Atoi(os.Args[3])
		if err != nil{
			fmt.Println(err.Error())
			os.Exit(1)		
		}
		cacheListWEC = wec.GenerateWECConfigs(cache)

	} else if (strings.ToLower(algorithm) == "wecv2") {
		// CP ratio
		cache, err := strconv.Atoi(os.Args[3])
		if err != nil{
			fmt.Println(err.Error())
			os.Exit(1)		
		}
		cacheListWEC = wec_v2.GenerateWECConfigs(cache)

	} else {
		cacheList, err = validateTraceSize(os.Args[3:])
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)		
		}
	}

	traces, err = readFile(filePath)
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}

	outPath = fmt.Sprintf("./output/%v_%v_%v.txt", time.Now().Unix(), algorithm, fs.Name())

	out, err = os.Create(outPath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer out.Close()

	if (strings.ToLower(algorithm) == "wec") {
		for _, cache := range cacheListWEC {
			simulator := wec.NewWEC(cache[0],cache[1],cache[2])
			timeStart = time.Now()
	
			for _, trace := range traces {
				err = simulator.Get(trace)
				if err != nil {
					log.Fatal(err.Error())
				}
			}
	
			simulator.PrintToFile(out, timeStart)
		}
	} else if (strings.ToLower(algorithm) == "wecv2") {
		for _, cache := range cacheListWEC {
			simulator := wec_v2.NewWEC(cache[0],cache[1],cache[2])
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

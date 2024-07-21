package wec_v5

import (
	"fmt"
	"ixtza/ajk/wec/simulator"
	"math"
	"os"
	"strings"
	"time"

	"github.com/secnot/orderedmap"
	"github.com/tidwall/btree"
)

type (
	WECData struct {
		address  int
		location string

		accessCount int
		lastAccess  int
		idleTime    int

		// spq bool
	}
	WECache struct {
		ramSize         int
		ssdSize         int
		hddSize         int
		wcqSize         int
		wcqSizeOriginal int

		candidateCount int
		requestCount   int

		hitCount  int
		missCount int

		writeCount int

		readRequestCount  int
		writeRequestCount int

		ssdHitCount int
		ramHitCount int

		wedPullThreshold  float32
		quitThreshold     int
		quitThresholdType string

		updatePeriode int

		WCQueue  *orderedmap.OrderedMap
		SPQueue  *orderedmap.OrderedMap
		RAMQueue *orderedmap.OrderedMap

		SSDMap  map[int]*WECData
		WCQTree *btree.Map[int, *orderedmap.OrderedMap]
	}
)

var peek any

func catch() {
	if r := recover(); r != nil {
		fmt.Println(peek)
		fmt.Println(r)
		panic("WEC Overflow")
	}
}

func New(
	capacitySize int,
	updatingPeriod int,
	quitThresholdType string,
	ramPercentage float32,
	capacitySizeRatio float32,
	wecDataThreshold float32,
) simulator.Simulator {

	var cacheSize int

	var ramSize int
	var ssdSize int
	var hddSize int
	var wcqSize int

	var candidateCount int
	var requestCount int

	var hitCount int
	var missCount int

	var writeCount int

	var ssdHitCount int
	var ramHitCount int

	var wedPullThreshold float32
	var quitThreshold int

	var updatePeriode int

	var WCQueue *orderedmap.OrderedMap
	var SPQueue *orderedmap.OrderedMap
	var RAMQueue *orderedmap.OrderedMap
	var SSDMap map[int]*WECData
	var WCQTree *btree.Map[int, *orderedmap.OrderedMap]

	hitCount = 0
	missCount = 0
	writeCount = 0
	ssdHitCount = 0
	ramHitCount = 0

	wedPullThreshold = wecDataThreshold
	quitThreshold = calculateQuitThreshold(quitThresholdType, capacitySizeRatio, capacitySize)

	updatePeriode = updatingPeriod

	cacheSize = int(float32(capacitySize) * capacitySizeRatio)

	ramSize = int(float32(cacheSize) * ramPercentage)
	ssdSize = cacheSize - ramSize
	hddSize = capacitySize

	wcqSize = cacheSize + int(float32(cacheSize)*0.1)

	WCQueue = orderedmap.NewOrderedMap()
	SPQueue = orderedmap.NewOrderedMap()
	RAMQueue = orderedmap.NewOrderedMap()

	SSDMap = map[int]*WECData{}
	WCQTree = btree.NewMap[int, *orderedmap.OrderedMap](2)

	return &WECache{
		ramSize:         ramSize,
		ssdSize:         ssdSize,
		hddSize:         hddSize,
		wcqSize:         wcqSize,
		wcqSizeOriginal: wcqSize,

		candidateCount: candidateCount,
		requestCount:   requestCount,

		hitCount:  hitCount,
		missCount: missCount,

		writeCount: writeCount,

		ssdHitCount: ssdHitCount,
		ramHitCount: ramHitCount,

		wedPullThreshold: wedPullThreshold,
		quitThreshold:    quitThreshold,

		quitThresholdType: quitThresholdType,

		updatePeriode: updatePeriode,

		WCQueue:  WCQueue,
		SPQueue:  SPQueue,
		RAMQueue: RAMQueue,
		SSDMap:   SSDMap,
		WCQTree:  WCQTree,
	}
}

func calculateQuitThreshold(
	quitThresholdType string,
	capacitySizeRatio float32,
	capacitySize int,
) int {
	switch quitThresholdType {
	case "cubic":
		return int(float64(capacitySize) * (math.Pow(float64(capacitySizeRatio), 3)))
	case "quadratic":
		return int(float64(capacitySize) * (math.Pow(float64(capacitySizeRatio), 2)))
	case "square_root":
		return int(float64(capacitySize) * math.Sqrt(float64(capacitySizeRatio)))
	case "cube_root":
		return int(float64(capacitySize) * math.Cbrt(float64(capacitySizeRatio)))
	default:
		return int(float32(capacitySize) * capacitySizeRatio)
	}
}

func (wec *WECache) ramReplace() (err error) {
	address, wecData, ok := wec.RAMQueue.GetFirst()
	if ok {
		data := wecData.(*WECData)
		if wec.RAMQueue.Len() > wec.ramSize {
			data.setLocation("HDD")
			wec.RAMQueue.Delete(address)
		}
	}
	return
}
func (wec *WECache) ramGetData(address int) (wecData *WECData) {
	data, _ := wec.RAMQueue.Get(address)
	wecData, _ = data.(*WECData)
	return
}

func (wec *WECache) wcqTreeUpsertData(accessCount, address int, data *WECData) (err error) {
	if data.accessCount > 1 {
		wec.wcqTreeRemoveData(data.accessCount-1, address)
	}
	wcqTreeData, wcqTreeDataExists := wec.WCQTree.Get(data.accessCount)
	if !wcqTreeDataExists {
		newMap := orderedmap.NewOrderedMap()
		newMap.Set(address, data)
		wec.WCQTree.Set(accessCount, newMap)
		wec.candidateCount += 1
	} else {
		wcqTreeData.Set(address, data)
		wec.candidateCount += 1
	}
	return
}
func (wec *WECache) wcqTreeRemoveData(accessCount, address int) (err error) {
	wcqTreeData, wcqTreeDataExists := wec.WCQTree.Get(accessCount)
	if !wcqTreeDataExists {
		return
	}
	if wcqTreeData == nil {
		wec.WCQTree.Delete(accessCount)
		return
	}
	wecData, wecDataExists := wcqTreeData.Get(address)
	if !wecDataExists {
		return
	}
	if wecData != nil {

		// wcqData := wec.wcqGetData(address)
		// if wcqData != nil {
		// 	wec.candidateCount -= 1
		// }

		wcqTreeData.Delete(address)
		wec.candidateCount -= 1
	}
	if wcqTreeData.Len() == 0 {
		wec.WCQTree.Delete(accessCount)
	}
	return
}
func (wec *WECache) wcqTreeFetchCandidate(weCandidateCount int) (datas []*WECData) {
	wec.WCQTree.Reverse(func(key int, value *orderedmap.OrderedMap) bool {
		iter := value.IterReverse()
		for _, wecData, ok := iter.Next(); ok; _, wecData, ok = iter.Next() {
			data, _ := wecData.(*WECData)
			if weCandidateCount == len(datas) {
				return false
			}
			datas = append(datas, data)
		}
		return true
	})
	return
}
func (wec *WECache) wcqEvict() (err error) {
	address, wecData, ok := wec.WCQueue.GetFirst()
	if ok {
		data := wecData.(*WECData)
		if data.location == "SSD" && wec.wcqSize < wec.WCQueue.Len() {
			wec.spqAddBlock(data.address, data)
			wec.WCQueue.Delete(address)
		} else if data.location == "HDD" && wec.wcqSize < wec.WCQueue.Len() {
			wec.wcqRemoveBlock(data)
		} else if data.location == "RAM" && wec.wcqSize < wec.WCQueue.Len() {
			wec.ramReplace()
			wec.wcqRemoveBlock(data)
		}
		wec.spqIncreaseIdleTime()
	}
	return
}
func (wec *WECache) wcqAddBlock(address int) (err error) {
	newBlock := &WECData{
		address:     address,
		accessCount: 1,
		lastAccess:  wec.requestCount,
		location:    "RAM",
		idleTime:    0,
		// spq:         false,
	}
	wec.WCQueue.Set(address, newBlock)
	wec.RAMQueue.Set(address, newBlock)
	wec.wcqTreeUpsertData(newBlock.accessCount, address, newBlock)
	return
}
func (wec *WECache) wcqRemoveBlock(data *WECData) (err error) {
	wec.wcqTreeRemoveData(data.accessCount, data.address)
	wec.WCQueue.Delete(data.address)
	return
}
func (wec *WECache) wcqGetData(address int) (wecData *WECData) {
	data, _ := wec.WCQueue.Get(address)
	wecData, _ = data.(*WECData)
	return
}
func (wec *WECache) wcqRequestReadHDD(address int, wecData *WECData) (err error) {
	wecData.setLocation("RAM")
	wec.RAMQueue.Set(address, wecData)
	wec.RAMQueue.MoveLast(address)
	return
}
func (wec *WECache) wcqRequestReadRAM(address int) (err error) {
	wec.RAMQueue.MoveLast(address)
	return
}

func (wec *WECache) spqIncreaseIdleTime() (err error) {
	iter := wec.SPQueue.Iter()
	for _, wecData, ok := iter.Next(); ok; _, wecData, ok = iter.Next() {
		data := wecData.(*WECData)
		data.idleTime += 1
		if data.idleTime > wec.quitThreshold {
			delete(wec.SSDMap, data.address)
			wec.SPQueue.Delete(data.address)
		}
	}
	return
}
func (wec *WECache) spqAddBlock(address int, wecData *WECData) (err error) {
	wecData.idleTime = wec.WCQueue.Len() - 1
	wec.SPQueue.Set(address, wecData)
	return
}
func (wec *WECache) spqGetData(address int) (wecData *WECData) {
	data, _ := wec.SPQueue.Get(address)
	wecData, _ = data.(*WECData)
	return
}
func (wec *WECache) spqRequestRead(address int, wecData *WECData) (err error) {
	wecData.idleTime = 0
	// wecData.spq = false
	wec.ssdHitCount += 1
	wec.SPQueue.Delete(address)
	return
}

func (wec *WECache) ssdUpdate() (err error) {

	var weCandidateData []*WECData
	var weCandidateCount int
	var ssdFreeSpace int
	var newWCQSize int

	ssdFreeSpace = wec.ssdSize - len(wec.SSDMap)
	// if ssdFreeSpace == 0 {
	// 	return
	// }

	weCandidateCount = int(math.Ceil(float64(wec.wedPullThreshold) * float64(wec.candidateCount)))

	weCandidateData = wec.wcqTreeFetchCandidate(weCandidateCount)

	if len(weCandidateData) > ssdFreeSpace {
		newWCQSize = wec.wcqSize - (len(weCandidateData) - ssdFreeSpace)
		if newWCQSize < wec.ramSize {
			wec.wcqSize = wec.ramSize
		}
	}

	if len(weCandidateData) < ssdFreeSpace {
		newWCQSize = wec.wcqSize + (ssdFreeSpace - len(weCandidateData))
	}

	if ssdFreeSpace > 0 {
		for i := 0; i < ssdFreeSpace && i < len(weCandidateData); i++ {
			wec.ssdAddBlock(weCandidateData[i])
			wec.wcqTreeRemoveData(weCandidateData[i].accessCount, weCandidateData[i].address)
		}
	}
	if newWCQSize < wec.wcqSize {
		deleteCount := wec.WCQueue.Len() - newWCQSize
		if deleteCount < 0 {
			deleteCount = 0
		}
		// ITERASI weCandidate DARI BELAKANG DAN DELETE DARI WCQueue & SSDMap SEBANYAK SELISIH KANDIDAT DENGAN FREE SSD
		// ITER WCQ DARI BELAKANG
		wec.wcqSize = newWCQSize
		for deleteCount != 0 {
			address, wecData, ok := wec.WCQueue.GetFirst()
			if ok {
				data := wecData.(*WECData)
				if data.location == "SSD" && wec.wcqSize < wec.WCQueue.Len() {
					wec.spqAddBlock(data.address, data)
					wec.WCQueue.Delete(address)
				} else if data.location == "HDD" && wec.wcqSize < wec.WCQueue.Len() {
					wec.wcqRemoveBlock(data)
				} else if data.location == "RAM" && wec.wcqSize < wec.WCQueue.Len() {
					wec.wcqRemoveBlock(data)
				}
				deleteCount -= 1
			}
		}
	} else {
		wec.wcqSize = newWCQSize
	}

	return
}
func (wec *WECache) ssdAddBlock(wecData *WECData) (err error) {
	wecData.setLocation("SSD")
	wec.writeCount += 1
	wec.SSDMap[wecData.address] = wecData
	return
}

func (wec *WECache) Get(trace simulator.Trace) (err error) {

	defer catch()

	wec.requestCount += 1

	address := trace.Addr
	request := strings.ToUpper(trace.Op)

	if wec.requestCount%wec.updatePeriode == 0 {
		res := wec.ssdUpdate()
		if res != nil {
			return res
		}
	}

	switch request {
	case "R":
		// FIND WCQ
		wec.readRequestCount += 1
		wcqData := wec.wcqGetData(address)
		if wcqData != nil {
			wcqData.accessCount += 1
			wec.WCQueue.MoveLast(address)
			if wcqData.location == "RAM" {
				// HANDLE WCQ READ RAM
				wec.wcqTreeUpsertData(wcqData.accessCount, wcqData.address, wcqData)
				wec.wcqRequestReadRAM(address)
				wec.hitCount += 1
			} else if wcqData.location == "HDD" {
				// HANDLE WCQ READ HDD
				wec.wcqTreeUpsertData(wcqData.accessCount, wcqData.address, wcqData)
				wec.missCount += 1
				wec.wcqRequestReadHDD(address, wcqData)
				wec.ramReplace()
			} else if wcqData.location == "SSD" {
				// HANDLE WCQ READ SSD
				wec.hitCount += 1
				wec.ssdHitCount += 1
			}
			return
		}
		// FIND SPQ
		spqData := wec.spqGetData(address)
		if spqData != nil {
			spqData.accessCount += 1
			wec.hitCount += 1
			// HANDLE SPQ READ SSD
			wec.spqRequestRead(address, spqData)
			wec.WCQueue.Set(address, spqData)
			wec.wcqEvict()
			return
		}
		// FIND RAM ADD ADITIONAL HITCOUNT
		ramData := wec.ramGetData(address)
		if ramData != nil {
			wec.ramHitCount += 1
		}
		wec.missCount += 1
		wec.wcqAddBlock(address)
		wec.ramReplace()
		wec.wcqEvict()
		return
	case "W":
		// FIND WCQ
		wec.writeRequestCount += 1
		wcqData := wec.wcqGetData(address)
		if wcqData != nil {
			if wcqData.location == "RAM" {
				wec.hitCount += 1
				wec.wcqRemoveBlock(wcqData)
			} else if wcqData.location == "SSD" {
				wec.hitCount += 1
				wec.ssdHitCount += 1
				wec.WCQueue.Delete(address)
			} else if wcqData.location == "HDD" {
				wec.missCount += 1
				wec.wcqRemoveBlock(wcqData)
			}
			return
		}
		// FIND SPQ
		spqData := wec.spqGetData(address)
		if spqData != nil {
			wec.hitCount += 1
			wec.ssdHitCount += 1
			wec.SPQueue.Delete(address)
			return
		}
		wec.missCount += 1
		return
	}

	return
}

func (wec *WECData) setLocation(location string) (err error) {
	wec.location = location
	return nil
}

func (wec *WECache) PrintToFile(file *os.File, timeStart time.Time) (err error) {
	duration := time.Since(timeStart)
	hitRatio := 100 * float32(float32(wec.hitCount)/float32(wec.hitCount+wec.missCount))
	writeEfficiency := float32(float32(wec.hitCount)/float32(wec.writeCount))
	cacheSize := wec.ssdSize + wec.ramSize
	result := fmt.Sprintf(`_______________________________________________________
WEC
cache size:%v
ssd size:%v
ram size:%v
hdd size:%v
quit threshold:%v
SSD hit:%v
RAM WCQ hit:%v
RAM Only hit:%v
cache hit:%v
cache miss:%v
hit ratio:%v
write efficiency:%v
write count:%v
write request count:%v
read request count:%v
duration:%v
!WEC|%v|%v|%v
`,
		cacheSize,
		wec.ssdSize,
		wec.ramSize,
		wec.hddSize,
		wec.quitThreshold,
		wec.ssdHitCount,
		wec.hitCount-wec.ssdHitCount,
		wec.ramHitCount,
		wec.hitCount,
		wec.missCount,
		hitRatio,
		writeEfficiency,
		wec.writeCount,
		wec.writeRequestCount,
		wec.readRequestCount,
		duration.Seconds(),
		cacheSize,
		wec.hitCount,
		wec.requestCount,
	)
	_, err = file.WriteString(result)
	return
}

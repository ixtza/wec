package wec

import (
	"os"
	"fmt"
	"container/list"
	"time"
	"math"

	"ixtza/ajk/wec/simulator"
	"github.com/secnot/orderedmap"
)

type (
	Node struct {
		op   string
		elem *list.Element
	}

	WECData struct {
		// metadata
		id          int
		location    string
		spq         bool
		accessCount int
		idleTime    int

		activeLifeSpan int // sum dari access interval/IRR
		averageAccessTime int // hasil activeLifeSpan / accessCount
	}

	WCQueue struct {
		ramSize      int
		ssdSize      int
		hddSize      int
		wcqSize      int
		hitCount     int
		missCount    int
		writeCount   int
		requestCount int
		spqCount int

		ssdFreeSpace int
		ssdHitCount int
		ramHitCount int

		wecThreshold float64
		quitThreshold int
		updatePeriode int

		WCQueue *orderedmap.OrderedMap
	}
)

// percobaan CR 1:20
// percobaan CR 1:10
// percobaan CR 3:20
// percobaan CR 1:5
// derajat f(CR) adalah quadrat
// percobaan sram 1:1
// percobaan sram 1:3
// percobaan sram 3:1

func NewWEC(ssdSize, ramSize, hddSize int) (wcq *WCQueue) {
	// menerima nilai ssdSize
	// menerima nilai ramSize
	// menerima nilai hddSize
	// hitung quitThreshold
	quitThreshold :=  int(float64(hddSize)*math.Sqrt((float64(ssdSize)+float64(ramSize))/float64(hddSize)))
	// hitung updatePeriode
	// updatePeriode := quitThreshold * 2
	// hitung wecSize awal
	wecSize := (ssdSize + ramSize)*(1 + int(math.Sqrt(float64((ssdSize+ramSize)/hddSize))))
	// wecSize := quitThreshold
	// updatePeriode := (ramSize + ssdSize) * 2
	updatePeriode := ssdSize*(1+1/2)
  // wecThreshold := int(float64(ssdSize)*math.Sqrt((float64(ssdSize)+float64(ramSize))/float64(hddSize)))
	return &WCQueue {
		ramSize: ramSize,
		ssdSize: ssdSize,
		ssdFreeSpace : ssdSize,
		hddSize: hddSize,
		wcqSize: wecSize,
		hitCount: 0,
		missCount: 0,
		writeCount: 0,
		requestCount: 0,
		spqCount: 0,
		// wecThreshold: wecThreshold,
		wecThreshold: 0.2,
		quitThreshold: quitThreshold,
		updatePeriode: 2500,
		WCQueue: orderedmap.NewOrderedMap(),
	}
}

func GenerateWECConfigs(storageSize int) (configs [][]int) {
	configs = [][]int{
		{(storageSize/20)/2, (storageSize/20)/2, storageSize},
		{(storageSize/20)/4, (storageSize/20*3)/4, storageSize},
		{(storageSize/20*3)/4, (storageSize/20)/4, storageSize},
		{(storageSize/10)/2, (storageSize/10)/2, storageSize},
		{(storageSize/10)/4, (storageSize/10*3)/4, storageSize},
		{(storageSize/10*3)/4, (storageSize/10)/4, storageSize},
		{(storageSize*3/20)/2, (storageSize*3/20)/2, storageSize},
		{(storageSize*3/20)/4, (storageSize*3/20*3)/4, storageSize},
		{(storageSize*3/20*3)/4, (storageSize*3/20)/4, storageSize},
		{(storageSize/5)/2, (storageSize/5)/2, storageSize},
		{(storageSize/5)/4, (storageSize/5*3)/4, storageSize},
		{(storageSize/5*3)/4, (storageSize/5)/4, storageSize},
	}
	return
}

func (wcq *WCQueue) UpdateSSDCache() (err error) {
	// Hapus & hitung data SSD yang sudah kadarluarsa sejumalah N
	// Hitung total data yang akan di promosikan sejumlah M
	// Masukan data WCQ dari peringkat 1 hingga N
	// Ubah wcqSize:
	// - JIKA KOSONG SSD N DAN PROMOSI N BLOCK => LEN WCQ = LEN WCQ
	// - JIKA KOSONG SSD N DAN PROMOSI N+M BLOCK => LEN WCQ = LEN WCQ - KOSONG SSD = LEN WCQ - N
	// - JIKA KOSONG SSD N+M DAN PROMOSI N BLCOK => LEN WCQ = LEN WCQ + M
	deleteCount := wcq.DeleteSSDExpiredData()
	totalFreeSpace := deleteCount + wcq.ssdFreeSpace
	if ( totalFreeSpace > 0) {
		wcq.wcqSize -=  deleteCount
		cacheCandidate := wcq.FetchCacheCandidates()
		newWCQSize := wcq.wcqSize + (deleteCount - len(cacheCandidate))
		wcq.SetWCQSize(newWCQSize)

		wcq.ssdFreeSpace -= (totalFreeSpace - len(cacheCandidate))

		traverseIndex := 0
		for traverseIndex < totalFreeSpace &&  traverseIndex < len(cacheCandidate){
			cacheCandidate[traverseIndex].SetLocation("SSD")
			wcq.writeCount += 1
			traverseIndex++
		}
	}
	return
}
func (wcq *WCQueue) DeleteSSDExpiredData() (count int) {
	iter := wcq.WCQueue.IterReverse()
	for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
		if (wecData.(*WECData).location == "SSD" && wecData.(*WECData).idleTime > wcq.quitThreshold) {
			wcq.WCQueue.Delete(id)
			count += 1
		}
	}
	return 
}
func (wcq *WCQueue) FetchCacheCandidates() (datas []*WECData){
	mapCandidate := make(map[int][]*WECData)
	maxAverage := -1
	count := 0
	iter := wcq.WCQueue.Iter()
	for _, wecData, ok := iter.Next(); ok; _, wecData, ok = iter.Next() {
		data := wecData.(*WECData)
		if (data.location == "HDD" || data.location == "RAM")  {
			if (data.averageAccessTime > maxAverage) { maxAverage = data.averageAccessTime }
			mapCandidate[data.averageAccessTime] = append(mapCandidate[data.averageAccessTime], data)
			count += 1
		}
	}

	for i := 0; i <= maxAverage || i <= int(float64(count)*float64(wcq.wecThreshold)) ; i++ {
		if (mapCandidate[i] != nil) {
			for block := range mapCandidate[i] {
				if (count == 0) { break }
				datas = append(datas, mapCandidate[i][block])
			}
		}
	}
	return
}
func (wcq *WCQueue) WCQEvict(targetId int) (err error) {
	// Iterasi dari belakang
	// Update idle time
	// Jika SSD idleTime lebih dari WCQ, set SPQ to TRUE
	// Jika RAM idleTime lebih dari ukuran RAM, move to HDD
	// Jika HDD idleTime lebih dari WCQ, EVICT
	iter := wcq.WCQueue.Iter()
	irr := 0
	for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
		if (id != targetId) {irr += 1}
		if (id == targetId) {
			wecData.(*WECData).activeLifeSpan += irr
			wecData.(*WECData).averageAccessTime = wecData.(*WECData).activeLifeSpan/wecData.(*WECData).accessCount
		}
		wecData.(*WECData).AdvanceIdleTime()
		if (wecData.(*WECData).location == "SSD" && wecData.(*WECData).idleTime > wcq.wcqSize) {
			wcq.spqCount += 1
			wecData.(*WECData).SetSQP(true)
			continue
		}
		if (wecData.(*WECData).location == "RAM" && wecData.(*WECData).idleTime > wcq.ramSize) {
			wecData.(*WECData).SetLocation("HDD")
		}
		// Ketika data ram demote ke HDD, tetap dilakukan identifikasi idleTime terhadap wcqSize
		if (wecData.(*WECData).location == "HDD" && wecData.(*WECData).idleTime > wcq.wcqSize) {
			wcq.WCQueue.Delete(id)
			continue
		}
	}
	return
}
func (wcq *WCQueue) RAMReplace() (err error) {
	// Iterasi dari belakang hingga ketemu data wec dengan lokasi RAM
	// Update lokasi data ke HDD
	iter := wcq.WCQueue.IterReverse()
	for _, wecData, ok := iter.Next(); ok; _, wecData, ok = iter.Next() {
		if (wecData.(*WECData).location == "SSD" && wecData.(*WECData).idleTime + 1 > wcq.ramSize) {
			wecData.(*WECData).SetLocation("HDD") 
		}
	}
	return
}
func (wcq *WCQueue) SetWCQSize(newSize int) (err error) {
	wcq.wcqSize = newSize
	return
}
func (wcq *WCQueue) SetUpdatePeriodeSize(newSize int) (err error) {
	wcq.updatePeriode = newSize
	return
}
func (wcq *WCQueue) Get(trace simulator.Trace) (err error) {

	id := trace.Addr
	request := trace.Op

	if (wcq.requestCount == wcq.updatePeriode) {
		res := wcq.UpdateSSDCache()
		if (res != nil) {
			return res
		}
	}

	wcq.requestCount += 1

	switch request {
	case "W":
		data := wcq.FindWCQData(id)
		if (data != nil) {
			wcq.WCQueue.Delete(id)
			return
		}
		break
	case "R":
		data := wcq.FindWCQData(id)
		if (data != nil) {
			data.AddAccessCount()
			data.ResetIdleTime()
			if (data.location == "RAM" || data.location == "SSD") {
				wcq.hitCount += 1
				if (data.location == "RAM") {wcq.ramHitCount += 1}
				if (data.location == "SSD") {wcq.ssdHitCount += 1}
				if (data.spq) {
					data.SetSQP(false)
					wcq.spqCount -= 1
				}
				data.idleTime = -1
				// pass block id untuk menghitung nilai IRR
				wcq.WCQEvict(id)
				wcq.WCQueue.MoveFirst(id)
				return
			}
			if (data.location == "HDD") {
				wcq.missCount += 1
				// pass block id untuk menghitung nilai IRR
				wcq.WCQEvict(id)
				wcq.WCQueue.MoveFirst(id)
				return
			}
		}
		if (data == nil) {
			wcq.missCount += 1
			wcq.WCQueue.Set(id, &WECData{
				id: id,
				accessCount: 1,
				spq: false,
				location: "RAM",
				idleTime: -1,
				activeLifeSpan: 0,
				averageAccessTime: -1,
			})
			wcq.RAMReplace()
			wcq.WCQEvict(id)
			return
		}
		break
	}
	return
}
func (wcq *WCQueue) FindWCQData(id int) (wecData *WECData) {
	data, _ := wcq.WCQueue.Get(id)
	wecData, _ = data.(*WECData)
	return
}
func (wcq *WCQueue) PrintToFile(file *os.File, timeStart time.Time) (err error) {
	duration := time.Since(timeStart)
	hitRatio := 100 * float32(float32(wcq.hitCount)/float32(wcq.hitCount+wcq.missCount))
	cacheSize := wcq.ssdSize + wcq.ramSize
	result := fmt.Sprintf(`_______________________________________________________
WEC
cache size : %v
ssd size : %v
ram size : %v
hdd size : %v
quit threshold : %v
SSD hit : %v
RAM hit : %v
cache hit : %v
cache miss : %v
hit ratio : %v
write count : %v
duration : %v
!WEC|%v|%v|%v
`, cacheSize, wcq.ssdSize, wcq.ramSize, wcq.hddSize, wcq.quitThreshold, wcq.ssdHitCount, wcq.ramHitCount, wcq.hitCount, wcq.missCount, hitRatio, wcq.writeCount, duration.Seconds(), cacheSize, wcq.hitCount, wcq.hitCount+wcq.missCount)
	_, err = file.WriteString(result)
	return
}

func (wec *WECData) AddAccessCount() (err error) {
	wec.accessCount += 1
	return
}
func (wec *WECData) AdvanceIdleTime() (err error) {
	wec.idleTime += 1
	return nil
}
func (wec *WECData) ResetIdleTime() (err error) {
	wec.idleTime = -1
	return nil
}
func (wec *WECData) SetSQP(value bool) (err error) {
	wec.spq = value
	return nil
}
func (wec *WECData) SetLocation(location string) (err error) {
	wec.location = location
	return nil
}

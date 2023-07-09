package wec_v2

import (
	"os"
	"fmt"
	"time"
	"math"

	"ixtza/ajk/wec/simulator"
	"github.com/secnot/orderedmap"
)

type (

	WECData struct {
		// metadata
		id          int
		location    string
		spq         bool
		accessCount int
		idleTime    int

		lastAccess int

		activeLifeSpan int // sum dari access interval/IRR
		averageAccessTime int // hasil activeLifeSpan / accessCount
	}

	WCQueue struct {
		totalAccess int

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
		SPQueue *orderedmap.OrderedMap
		RAMQueue *orderedmap.OrderedMap
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
	// hitung wecSize awal
	wecSize := (ssdSize + ramSize)*(1 + int(math.Sqrt((float64(ssdSize)+float64(ramSize))/float64(hddSize))))
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
		wecThreshold: 0.1,
		quitThreshold: quitThreshold,
		updatePeriode: 50,
		WCQueue: orderedmap.NewOrderedMap(),
		SPQueue: orderedmap.NewOrderedMap(),
		RAMQueue: orderedmap.NewOrderedMap(),
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
func (wcq *WCQueue) Get(trace simulator.Trace) (err error) {

	wcq.requestCount += 1

	id := trace.Addr
	request := trace.Op

	if (wcq.requestCount % wcq.updatePeriode == 0) {
		res := wcq.UpdateSSDCache()
		if (res != nil) {
			return res
		}
	}


	switch request {
	case "W":
		data := wcq.FindWCQData(id)
		if (data != nil) {
			wcq.WCQueue.Delete(id)
			return
		}
		if (data == nil) {
			data = wcq.FindSPQData(id)
			if (data != nil) {
				wcq.SPQueue.Delete(id)
				return
			}
		}
		break
	case "R":
		data := wcq.FindWCQData(id)
		if (data == nil) {
			data = wcq.FindSPQData(id)
		}
		if (data != nil) {
			if (data.location == "SSD") {
				wcq.ssdHitCount += 1
				if (data.spq) {
					wcq.requestCount += 1
					wcq.SPQueue.Delete(data.id);
					wcq.WCQEvict()
					wcq.WCQueue.Set(data.id,data);
				}
				data.SetSQP(false)
			}
			if (data.location == "RAM") {
				wcq.RAMQueue.MoveLast(id)
				wcq.ramHitCount += 1
			}
			if (data.location == "SSD" || data.location == "RAM") {
				wcq.requestCount -= 1
				wcq.hitCount += 1
			}
			if (data.location == "HDD") {
				data.SetLocation("RAM")
				wcq.RAMReplace()
				wcq.RAMQueue.Set(id,data)
				wcq.missCount += 1
			}
			wcq.UpdateIRR(id)
			data.accessCount += 1
			data.lastAccess = wcq.requestCount
			data.SetIdleTime(0)
			wcq.WCQueue.MoveLast(id)
			return 
		} else {
			wcq.missCount += 1
			newBlock := &WECData{
				id: id,
				accessCount: 1,
				lastAccess: wcq.requestCount,
				spq: false,
				location: "RAM",
				idleTime: 0,
				activeLifeSpan: 0,
				averageAccessTime: wcq.hddSize,
			}
			wcq.WCQueue.Set(id, newBlock)
			wcq.RAMQueue.Set(id, newBlock)
			wcq.RAMReplace()
			wcq.WCQEvict()
			return 
		}
		// Jika WEC Penuh
		// Jika WEC Tidak Penuh
	}
	return
}

func (wcq *WCQueue) WCQEvict() (err error) {
	// Iterasi dari belakang
	// Update idle time
	// Jika SSD idleTime lebih dari WCQ, set SPQ to TRUE
	// Jika RAM idleTime lebih dari ukuran RAM, move to HDD
	// Jika HDD idleTime lebih dari WCQ, EVICT
	id,data,ok := wcq.WCQueue.GetFirst()
	if (ok) {
		if (data.(*WECData).location == "SSD" && ( wcq.requestCount - data.(*WECData).lastAccess) > wcq.wcqSize) {
			data.(*WECData).SetSQP(true)
			wcq.SPQueue.Set(id,data.(*WECData))
			wcq.WCQueue.Delete(id)
		}
		if (data.(*WECData).location == "HDD" && ( wcq.requestCount - data.(*WECData).lastAccess) > wcq.wcqSize) {
			wcq.WCQueue.Delete(id)
		}
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
	wcq.DeleteSSDExpiredData()
	if ( wcq.ssdFreeSpace > 0 ) {
		// set new WCQ size
		cacheCandidate := wcq.FetchCacheCandidates()

		newWCQSize := wcq.wcqSize + (wcq.ssdFreeSpace - len(cacheCandidate))

		if (newWCQSize > wcq.quitThreshold) { newWCQSize = wcq.quitThreshold }
		// jika newWCQSize < wcq.wcqSize iterasi dari belakang untuk hapus WCQ
		if (newWCQSize < wcq.wcqSize) {
			if ( newWCQSize < 0 ) { 
				fmt.Println(newWCQSize , wcq.ssdFreeSpace)
				newWCQSize = 0
			 }
			iter := wcq.WCQueue.IterReverse()
			for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
				if ((wcq.requestCount - wecData.(*WECData).lastAccess) > newWCQSize && wecData.(*WECData).location == "HDD") {
					wcq.WCQueue.Delete(id)
				}
				if ((wcq.requestCount - wecData.(*WECData).lastAccess) > newWCQSize && wecData.(*WECData).location == "SSD") {
					wecData.(*WECData).SetSQP(true)
					wcq.SPQueue.Set(id,wecData.(*WECData))
					wcq.WCQueue.Delete(id)
					wcq.spqCount++
				}
				if (newWCQSize == wcq.wcqSize) { break }
			}
		}
		// jika ukuran baru besar dari sama dengan
		// lakukan promote
		wcq.SetWCQSize(newWCQSize)


		traverseIndex := 0
		for traverseIndex < wcq.ssdFreeSpace {
			if (traverseIndex == wcq.ssdFreeSpace || traverseIndex == len(cacheCandidate)) { break }
			cacheCandidate[traverseIndex].SetLocation("SSD")
			wcq.writeCount += 1
			wcq.ssdFreeSpace--
			traverseIndex++
		}

	}
	return
}
func (wcq *WCQueue) RAMReplace() (err error) {
	// ambil block terakhir dari ramqueue
	// jika last access - accesscount > ram zise 
	// - delete dari ram queue 
	// - set lokasi ke HDD
	if (wcq.RAMQueue.Len() > wcq.ramSize) {
		iter := wcq.RAMQueue.Iter()
		for id, wecData, ok := iter.Next(); ok && ((wcq.requestCount - wecData.(*WECData).lastAccess) >= wcq.ramSize); id, wecData, ok = iter.Next() {
			data := wecData.(*WECData)
			wcq.RAMQueue.Delete(id)
			if ((wcq.requestCount - data.lastAccess > wcq.wcqSize)) { wcq.WCQueue.Delete(id) }
			data.SetLocation("HDD")
		}
	} else {
		id,data,ok := wcq.RAMQueue.GetFirst()
		if (ok && (wcq.requestCount - data.(*WECData).lastAccess > wcq.ramSize)) {
			data.(*WECData).SetLocation("HDD")
			wcq.RAMQueue.Delete(id)
		}
	}
	return
}
func (wcq *WCQueue) DeleteSSDExpiredData() (count int) {
	iter := wcq.SPQueue.Iter()
	for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
		if ((wcq.requestCount - wecData.(*WECData).lastAccess) > wcq.quitThreshold &&  wecData.(*WECData).location == "SSD") {
			wcq.SPQueue.Delete(id)
			wcq.ssdFreeSpace++
			count += 1
		}
	}
	return 
}
func (wcq *WCQueue) FetchCacheCandidates() (datas []*WECData){
	mapCandidate := make(map[int][]*WECData)
	maxAverage := -1
	count := 0
	iter := wcq.WCQueue.IterReverse()
	for _, wecData, ok := iter.Next(); ok; _, wecData, ok = iter.Next() {
		data := wecData.(*WECData)
		if ( data.averageAccessTime == 0 ) {
			mapCandidate[-1] = append(mapCandidate[-1], data)
			count += 1
			continue
		} else { 
			ratio := data.activeLifeSpan/data.averageAccessTime 
			if (data.location == "HDD" || data.location == "RAM")  {
				if (ratio > maxAverage) { maxAverage = ratio }
				mapCandidate[ratio] = append(mapCandidate[ratio], data)
				count += 1
			}
		}
	}

	for i := maxAverage; len(datas) <= int(float64(count)*float64(wcq.wecThreshold)) ; i-- {
		if (len(mapCandidate[-1]) > 0) {
			for block := range mapCandidate[-1] {
				if (count == 0) { break }
				datas = append(datas, mapCandidate[-1][block])
				count--
			}
		}
		if (count == 0) { break }
		if (mapCandidate[i] != nil) {
			for block := range mapCandidate[i] {
				if (count == 0) { break }
				datas = append(datas, mapCandidate[i][block])
				count--
			}
		}
	}
	return
}
func (wcq *WCQueue) UpdateIRR(targetId int)(err error) {
	iter := wcq.WCQueue.IterReverse()
	irr := 0
	for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
		if (id != targetId) {irr += 1}
		if (id == targetId) {
			wecData.(*WECData).activeLifeSpan += irr
			wecData.(*WECData).averageAccessTime = wecData.(*WECData).activeLifeSpan/wecData.(*WECData).accessCount
			break
		}
	}
	iter = wcq.SPQueue.IterReverse()
	for id, wecData, ok := iter.Next(); ok; id, wecData, ok = iter.Next() {
		if (id != targetId) {irr += 1}
		if (id == targetId) {
			wecData.(*WECData).activeLifeSpan += irr
			wecData.(*WECData).averageAccessTime = wecData.(*WECData).activeLifeSpan/wecData.(*WECData).accessCount
			break
		}
	}
	return
}
func (wcq *WCQueue) SetWCQSize(newSize int) (err error) {
	wcq.wcqSize = newSize
	return
}
func (wcq *WCQueue) FindWCQData(id int) (wecData *WECData) {
	data, _ := wcq.WCQueue.Get(id)
	wecData, _ = data.(*WECData)
	return
}
func (wcq *WCQueue) FindSPQData(id int) (wecData *WECData) {
	data, _ := wcq.SPQueue.Get(id)
	wecData, _ = data.(*WECData)
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
func (wec *WECData) SetIdleTime(newIdleTime int) (err error) {
	wec.idleTime = newIdleTime
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
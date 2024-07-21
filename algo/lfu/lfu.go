package lfu

import (
	"container/list"
	"fmt"
	"os"
	"time"

	"ixtza/ajk/wec/simulator"
	// "github.com/esaiy/golang-lirs/simulator"
	"github.com/petar/GoLLRB/llrb"
)

const MAXFREQ = 1000

type (
	Node = struct {
		lba  int
		freq int
		op   string
		elem *list.Element
	}
	LFU struct {
		maxlen      int
		available   int
		totalaccess int
		hit         int
		miss        int
		pagefault   int
		write       int

		tlba    *llrb.LLRB
		freqArr [MAXFREQ]*list.List
	}
)

func NewLFU(cacheSize int) *LFU {
	lfu := &LFU{
		maxlen:      cacheSize,
		available:   cacheSize,
		totalaccess: 0,
		hit:         0,
		miss:        0,
		pagefault:   0,
		write:       0,
		tlba:        llrb.New(),
		freqArr:     [MAXFREQ]*list.List{},
	}
	for i := 0; i < MAXFREQ; i++ {
		lfu.freqArr[i] = list.New()
	}
	return lfu
}

type NodeLba Node

func (x *NodeLba) Less(than llrb.Item) bool {
	return x.lba < than.(*NodeLba).lba
}

func (lfu *LFU) put(data *NodeLba) (exists bool) {
	var el *list.Element
	kk := new(NodeLba)

	node := lfu.tlba.Get((*NodeLba)(data)) // coba ambil, apa ada ?
	if node != nil {
		lfu.hit++
		dd := node.(*NodeLba) // shortcut saja
		if data.op == "W" {
			lfu.write++
		}
		if dd.freq < MAXFREQ { // wes mentok ?
			lst := lfu.freqArr[dd.freq-1]
			lst.Remove(dd.elem) // mau dipindah
			dd.freq++
			lst = lfu.freqArr[dd.freq-1]
			el = lst.PushFront(dd.elem.Value) // sip iki
			dd.elem = el                      // update elem
		}
		return true
	} else { // not exist
		lfu.miss++
		lfu.write++
		if lfu.available > 0 {
			lfu.available--
			el := lfu.freqArr[0].PushFront(data) // selalu 1 khan ?
			data.elem = el
			lfu.tlba.InsertNoReplace(data)
		} else {
			lfu.pagefault++
			el = nil
			for ii := 0; ii < MAXFREQ; ii++ { // cari list yang tidak kosong, terus buang
				if lfu.freqArr[ii].Len() > 0 {
					el = lfu.freqArr[ii].Back() // ambil yang paling lama
					lba := el.Value.(*NodeLba).lba
					kk.lba = lba
					lfu.tlba.Delete(kk) // hapus dah
					lfu.freqArr[ii].Remove(el)
					break
				}
			}
			//fmt.Printf("len %d:%d    vs", lfu.available, lfu.tlba.Len())

			//if el == nil {
			//println("ndak mungkinnn")
			//}
			el = lfu.freqArr[0].PushFront(data)
			data.elem = el
			lfu.tlba.InsertNoReplace(data)
			//fmt.Printf("     %d:%d\n", cache.totalaccess, cache.tlba.Len())
		}
		return false
	}
}

func (lfu *LFU) Get(trace simulator.Trace) (err error) {
	lfu.totalaccess++
	obj := new(NodeLba)
	obj.lba = trace.Addr
	obj.op = trace.Op
	obj.freq = 1

	lfu.put(obj)

	return nil
}
func (lfu LFU) PrintToFile(file *os.File, timeStart time.Time) (err error) {
	sum := 0
	for ii := 0; ii < MAXFREQ; ii++ {
		sum = sum + lfu.freqArr[ii].Len()
	}
	file.WriteString("------------------------------------\n")
	file.WriteString(fmt.Sprintf("NUM ACCESS: %d\n", lfu.totalaccess))
	file.WriteString(fmt.Sprintf("cache size: %d\n", lfu.maxlen))
	file.WriteString(fmt.Sprintf("cache hit: %d\n", lfu.hit))
	file.WriteString(fmt.Sprintf("cache miss: %d\n", lfu.miss))
	file.WriteString(fmt.Sprintf("ssd write: %d\n", lfu.write))
	file.WriteString(fmt.Sprintf("write efficiency : %d\n", (lfu.hit / lfu.write)))
	file.WriteString(fmt.Sprintf("hit ratio : %8.4f\n", (float64(lfu.hit)/float64(lfu.totalaccess))*100))
	file.WriteString(fmt.Sprintf("isi tree %d\n", lfu.tlba.Len()))
	file.WriteString(fmt.Sprintf("isi array: %d\n", sum))

	file.WriteString(fmt.Sprintf("!LFU|%d|%d|%d\n", lfu.maxlen, lfu.hit, lfu.write))

	return nil
}

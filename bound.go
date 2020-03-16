package fetcher

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"
)

type DBAddress struct {
	IP         string
	StatusPort uint64
	Local      bool

	mayProc *process.Process
}

type Bound interface {
	Record(dbAddresses []DBAddress)
	CheckDuration() time.Duration
}

type SpeedBound struct {
	DeltaSecs uint64
	DeltaMB   uint64

	lastMap sync.Map
}

func NewSpeedBound(DeltaSecs, DeltaMB uint64) Bound {
	return SpeedBound{
		DeltaSecs: DeltaSecs,
		DeltaMB:   DeltaMB,
		lastMap:   sync.Map{},
	}
}

func (s SpeedBound) Record(dbAddresses []DBAddress) {
	var wg sync.WaitGroup
	for _, v := range dbAddresses {
		currentAddress := v
		wg.Add(1)
		go func() {
			defer wg.Done()
			currentMem := FetchMemory(currentAddress)
			m, ok := s.lastMap.Load(currentAddress)
			if ok {
				lastMem := m.(uint64)
				if currentMem-lastMem > s.DeltaMB {
					FetchFlameGraph(currentAddress)
				}
			}
			s.lastMap.Store(currentAddress, currentMem)
		}()
	}
	wg.Wait()
}

func (s SpeedBound) CheckDuration() time.Duration {
	return time.Second * time.Duration(s.DeltaSecs)
}

func NewQuantityBound(prop float64) Bound {
	return &QuantityBound{Proportion: prop}
}

type QuantityBound struct {
	Proportion float64
}

func (q QuantityBound) Record(dbAddresses []DBAddress) {
	var wg sync.WaitGroup
	for _, v := range dbAddresses {
		currentAddress := v
		wg.Add(1)
		go func() {
			mem := FetchMemory(currentAddress)
			avail := FetchAvailable(currentAddress)
			if float64(avail)*q.Proportion <= float64(mem) {
				FetchFlameGraph(currentAddress)
			}
		}()
	}
	wg.Wait()
}

func (q QuantityBound) CheckDuration() time.Duration {
	return 30 * time.Second
}

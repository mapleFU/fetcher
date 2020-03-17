package fetcher

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"
)

type DBAddress struct {
	IP         string `json:"ip"`
	StatusPort uint64 `json:"status_port" yaml:"status_port"`
	Local      bool   `json:"local"`
	User       string `json:"user,omitempty"`

	mayProc *process.Process
}

type Bound interface {
	Record(dbAddresses []DBAddress, user string, saveDir string)
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

func (s SpeedBound) Record(dbAddresses []DBAddress, user string, saveDir string) {
	var wg sync.WaitGroup
	for _, v := range dbAddresses {
		currentAddress := v
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, currentMem := FetchMemoryAndAvailable(currentAddress, user)
			m, ok := s.lastMap.Load(currentAddress)
			if ok {
				lastMem := m.(uint64)
				if currentMem-lastMem > s.DeltaMB {
					FetchFlameGraph(currentAddress, saveDir)
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

func (q QuantityBound) Record(dbAddresses []DBAddress, user string, saveDir string) {
	var wg sync.WaitGroup
	ipcounter := map[string]int{}

	for _, v := range dbAddresses {
		ipcounter[v.IP] = ipcounter[v.IP] + 1
	}
	for _, v := range dbAddresses {
		currentAddress := v
		wg.Add(1)
		go func() {
			avail, mem := FetchMemoryAndAvailable(currentAddress, user)
			if float64(avail)*q.Proportion <= float64(mem) * float64(ipcounter[currentAddress.IP]) {
				FetchFlameGraph(currentAddress, saveDir)
			}
		}()
	}
	wg.Wait()
}

func (q QuantityBound) CheckDuration() time.Duration {
	return 90 * time.Second
}

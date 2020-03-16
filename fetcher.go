package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
)

func FetchFlameGraph(address DBAddress) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/debug/pprof/profile", address.IP, address.StatusPort))
	if err != nil {
		log.Warnf("Get flameGraph from %s error", address, err.Error())
		return
	}

	defer resp.Body.Close()
	fileName := fmt.Sprintf("prof-%s", time.Now().Format("Jan-02-2006"))
	f, err := os.Create(fileName)
	if err != nil {
		log.Warnf("Create file %s error", fileName)
	}
	defer f.Close()
	io.Copy(f, resp.Body)
}

func FetchMemory(address DBAddress) uint64 {
	if address.Local {
		p, err := process.Processes()
		if err != nil {
			log.Warn("Failed to get processes")
		}
		if address.mayProc == nil {
			for _, proc := range p {
				if strings.Contains(proc.String(), "tidb-server") {
					address.mayProc = proc
					break
				}
			}
		}
		currentProc := address.mayProc
		m, err := currentProc.MemoryInfo()
		if err != nil {
			return 0
		}
		return m.Data / (1024 * 1024)
	} else {
		log.Warn("Unimplemented FetchAvailable when it's not Local")
		return 0
	}
}

func FetchAvailable(address DBAddress) uint64 {
	if address.Local {
		v, _ := mem.VirtualMemory()
		return v.Total
	} else {
		log.Warn("Unimplemented FetchAvailable when it's not Local")
		return 0
	}
}

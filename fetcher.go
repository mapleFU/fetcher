package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/eugenmayer/go-sshclient/sshwrapper"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
)

func FetchFlameGraph(address DBAddress, saveDir string) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/debug/pprof/profile", address.IP, address.StatusPort))
	if err != nil {
		log.Warnf("Get flameGraph from %s error", address, err.Error())
		return
	}

	defer resp.Body.Close()
	ipstr := strings.ReplaceAll(address.IP, ".", "-")
	fileName := path.Join(saveDir, fmt.Sprintf("%s-prof-%d", ipstr, time.Now().Unix()))
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
				name, err := proc.Name()
				if err != nil {
					continue
				}
				if strings.Contains(name, "tidb-server") {
					address.mayProc = proc
					break
				}
			}
			if address.mayProc == nil {
				log.Warn("NO TIDB!")
				return 0
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
		log.Info("Total is ", v.Total)
		return v.Total / (1024 * 1024)
	} else {
		log.Warn("Unimplemented FetchAvailable when it's not Local")
		return 0
	}
}

func FetchMemoryAndAvailable(address DBAddress, user string) (uint64, uint64) {
	if address.Local {
		return FetchMemory(address), FetchAvailable(address)
	} else {
		return remoteSSHFetch(address, user)
	}
}

func remoteSSHFetch(address DBAddress, user string) (uint64, uint64) {
	sshApi, err := sshwrapper.DefaultSshApiSetup(address.IP, 22, user, os.Getenv("HOME")+"/.ssh/id_rsa")
	if err != nil {
		log.Fatal(err)
	}

	var resp, errResp string
	if resp, errResp, err = sshApi.Run("awk '/Mem:/ {print $2}' <(free -m)"); err != nil {
		log.Warn("Failed to run: "+err.Error(), errResp)
		return 0, 0
	}
	total, err := strconv.Atoi(resp)
	if err != nil {
		log.Warn("Failed to run: " + err.Error())
		return 0, 0
	}

	if resp, errResp, err = sshApi.Run("ps -aux | grep tidb-server | awk {'print $5\" \"$11'}"); err != nil {
		log.Warn("Failed to run: "+err.Error(), errResp)
		return 0, 0
	}

	var current uint64
	avails := strings.Split(resp, "\r\n")
	for _, v := range avails {
		arr := strings.Split(v, " ")
		if len(arr) < 2 {
			continue
		}
		if strings.Contains(arr[1], "grep") {
			continue
		}
		val, err := strconv.Atoi(arr[0])
		if err != nil {
			continue
		}
		current = uint64(val)
	}

	return uint64(total), current
}

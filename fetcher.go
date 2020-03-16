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
	log "github.com/sirupsen/logrus"
)

func FetchFlameGraph(address DBAddress, saveDir string) {
	getAddress := fmt.Sprintf("http://%s:%d/debug/pprof/profile", address.IP, address.StatusPort)
	resp, err := http.Get(getAddress)
	if err != nil {
		log.Warnf("Get flameGraph from %s error", getAddress, err.Error())
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

func FetchMemoryAndAvailable(address DBAddress, user string) (uint64, uint64) {
	return remoteSSHFetch(address, user)
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
	resp = strings.Trim(resp, "\r\n")
	total, err := strconv.Atoi(resp)
	if err != nil {
		log.Warn("Failed to run: " + err.Error())
		return 0, 0
	}

	if resp, errResp, err = sshApi.Run("ps -aux | grep tidb-server | awk '{print $5\" \"$11;$0=$0} NF=NF'"); err != nil {
		log.Warn("Failed to run: "+err.Error(), errResp)
		return 0, 0
	}
	originResp := resp
	resp = strings.Trim(resp, "\r\n")

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

		statusCheckPass := false
		statusString := fmt.Sprintf("status=%d", address.StatusPort)
		// default status_port, no status or status=10080 is available
		if address.StatusPort == 10080 {
			if !strings.Contains(originResp, statusString) {
				statusCheckPass = true
			}
		}
		// otherwise, there should be a --status=status_port string
		if !statusCheckPass {
			statusCheckPass = strings.Contains(originResp, statusString)
		}

		val, err := strconv.Atoi(arr[0])
		if err != nil {
			continue
		}
		current = uint64(val)
	}

	return uint64(total), current
}

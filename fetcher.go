package fetcher

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func FetchFlameGraph(address DBAddress, saveDir string) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/debug/pprof/profile", address.IP, address.StatusPort))
	if err != nil {
		log.Warnf("Get flameGraph from %s error", address, err.Error())
		return
	}

	defer resp.Body.Close()
	fileName := path.Join(saveDir, fmt.Sprintf("%s-prof-%s", address.IP, time.Now().Format("2012-11-01T22:08:41+00:00")))
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
	hostKey := getHostKey("")
	cfg := ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		// optional host key algo list
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoRSA,
			ssh.KeyAlgoDSA,
			ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoECDSA384,
			ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoED25519,
		},
		// optional tcp connect timeout
		Timeout: 5 * time.Second,
	}
	client, err := ssh.Dial("tcp", address.IP+":22", &cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// start session
	sess, err := client.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	// setup standard out and error
	// uses writer interface

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	var b bytes.Buffer
	sess.Stdout = &b
	if err := sess.Run("awk '/Mem:/ {print $2}' <(free -m)"); err != nil {
		log.Warn("Failed to run: " + err.Error())
		return 0, 0
	}
	total, err := strconv.Atoi(b.String())
	if err != nil {
		log.Warn("Failed to run: " + err.Error())
		return 0, 0
	}
	b.Truncate(0)

	if err := sess.Run("ps -aux | grep tidb-server | awk {'print $5\" \"$11'}"); err != nil {
		log.Warn("Failed to run: " + err.Error())
		return 0, 0
	}
	var current uint64
	avails := strings.Split(b.String(), "\n")
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

func getHostKey(host string) ssh.PublicKey {
	// parse OpenSSH known_hosts file
	// ssh or use ssh-keyscan to get initial key
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], host) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				log.Fatalf("error parsing %q: %v", fields[2], err)
			}
			break
		}
	}

	if hostKey == nil {
		log.Fatalf("no hostkey found for %s", host)
	}

	return hostKey
}

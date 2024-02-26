package hooks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cloudflare/cfssl/log"
)

const mockTable string = "mock"
const configMockTable string = "configMock"
const mockTableIndex string = "id"
const configMockTableIndex string = "id"
const mockTableIndexField string = "Id"
const configMockTableIndexField string = "Id"

// ConvertIPToUint32 converts a string representation of an IPv4 address to a 32-bit integer.
func ConvertIPToUint32(ipStr string) (uint32, error) {
	ipAddr := net.ParseIP(ipStr)
	if ipAddr != nil {
		ipAddr = ipAddr.To4()
		if ipAddr != nil {
			return binary.BigEndian.Uint32(ipAddr), nil
		} else {
			return 0, errors.New("not a valid IPv4 address")
		}
	} else {
		return 0, errors.New("failed to parse IP address")
	}
}

func GetPIDByPort(port int) (int, error) {
	// Run the lsof command to find the process using the given port
	cmd := exec.Command("lsof", "-n", "-i", fmt.Sprintf(":%d", port))
	
	log.Debug("Getting pid using port", cmd)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	// Parse the output of lsof
	lines := strings.Split(out.String(), "\n")
	if len(lines) > 1 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 2 {
			pid, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0, err
			}
			return pid, nil
		}
	}

	// If we get here, no process was found using the given port
	return 0, fmt.Errorf("no process found using port %d", port)
}

package usbpcap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"usb-suspend-watch/internal/model"
)

const (
	defaultSnapshotLength = "65535"
	defaultBufferLength   = "1048576"
)

type Interface struct {
	Value   string
	Display string
	Devices []Device
}

type Device struct {
	Value   string
	Address int
	Display string
	Parent  string
	Enabled bool
}

type Plan struct {
	ExePath          string
	Interface        Interface
	OutputPath       string
	MetadataPath     string
	Args             []string
	DeviceAddresses  []int
	MatchReasons     []string
	CaptureAll       bool
	TargetSummary    string
	Warning          string
	DiscoverySummary string
}

func DiscoverExecutable() (string, []string, error) {
	var tried []string
	if override := strings.TrimSpace(os.Getenv("USBPCAP_CMD")); override != "" {
		tried = append(tried, override)
		if fileExists(override) {
			return override, tried, nil
		}
		return "", tried, fmt.Errorf("USBPCAP_CMD points to a missing file: %s", override)
	}
	if path, err := exec.LookPath("USBPcapCMD.exe"); err == nil {
		return path, append(tried, "PATH:USBPcapCMD.exe"), nil
	}
	tried = append(tried, "PATH:USBPcapCMD.exe")
	for _, path := range commonExecutablePaths() {
		tried = append(tried, path)
		if fileExists(path) {
			return path, tried, nil
		}
	}
	return "", tried, errors.New("USBPcapCMD.exe was not found")
}

func commonExecutablePaths() []string {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	if programFilesX86 == "" {
		programFilesX86 = `C:\Program Files (x86)`
	}
	roots := []string{programFiles, programFilesX86}
	var paths []string
	for _, root := range roots {
		paths = append(paths,
			filepath.Join(root, "USBPcap", "USBPcapCMD.exe"),
			filepath.Join(root, "Wireshark", "extcap", "USBPcapCMD.exe"),
			filepath.Join(root, "Wireshark", "extcap", "wireshark", "USBPcapCMD.exe"),
			filepath.Join(root, "Wireshark", "USBPcapCMD.exe"),
		)
	}
	return paths
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DiscoverInterfaces(exePath string) ([]Interface, string, error) {
	output, err := runCommand(exePath, "--extcap-interfaces")
	if err != nil {
		return nil, string(output), fmt.Errorf("list USBPcap interfaces: %w", err)
	}
	interfaces := ParseInterfaces(string(output))
	for i := range interfaces {
		configOutput, configErr := runCommand(exePath, "--extcap-interface", interfaces[i].Value, "--extcap-config")
		if configErr != nil {
			continue
		}
		interfaces[i].Devices = ParseConfigDevices(string(configOutput))
	}
	return interfaces, string(output), nil
}

func runCommand(exePath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, exePath, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, ctx.Err()
	}
	return output, err
}

var braceFieldRe = regexp.MustCompile(`\{([^=]+)=([^}]*)\}`)

func ParseInterfaces(output string) []Interface {
	var interfaces []Interface
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "interface ") {
			continue
		}
		fields := parseBraceFields(line)
		value := fields["value"]
		if value == "" {
			continue
		}
		interfaces = append(interfaces, Interface{
			Value:   value,
			Display: fields["display"],
		})
	}
	return interfaces
}

func ParseConfigDevices(output string) []Device {
	var devices []Device
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "value ") {
			continue
		}
		fields := parseBraceFields(line)
		value := fields["value"]
		if value == "" {
			continue
		}
		devices = append(devices, Device{
			Value:   value,
			Address: leadingAddress(value),
			Display: fields["display"],
			Parent:  fields["parent"],
			Enabled: strings.EqualFold(fields["enabled"], "true"),
		})
	}
	return devices
}

func parseBraceFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, match := range braceFieldRe.FindAllStringSubmatch(line, -1) {
		if len(match) == 3 {
			fields[match[1]] = match[2]
		}
	}
	return fields
}

func leadingAddress(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(value[:end])
	return n
}

func BuildPlan(exePath string, interfaces []Interface, target model.DeviceSnapshot, outputPath, metadataPath string) (Plan, error) {
	if len(interfaces) == 0 {
		return Plan{}, errors.New("USBPcap exposes no capture interfaces")
	}
	bestInterface, addresses, reasons := bestInterfaceMatch(interfaces, target)
	if bestInterface.Value == "" {
		if len(interfaces) > 1 {
			return Plan{}, fmt.Errorf("target was not found in USBPcap extcap device lists across %d interfaces; refusing to guess the Root Hub", len(interfaces))
		}
		bestInterface = interfaces[0]
	}
	plan := Plan{
		ExePath:          exePath,
		Interface:        bestInterface,
		OutputPath:       outputPath,
		MetadataPath:     metadataPath,
		DeviceAddresses:  addresses,
		MatchReasons:     reasons,
		CaptureAll:       len(addresses) == 0,
		TargetSummary:    target.DisplayName(),
		DiscoverySummary: fmt.Sprintf("%d USBPcap interface(s), %d candidate device row(s)", len(interfaces), countDevices(interfaces)),
	}
	args := []string{"-d", bestInterface.Value, "-o", outputPath, "-s", defaultSnapshotLength, "-b", defaultBufferLength, "--inject-descriptors"}
	if len(addresses) > 0 {
		args = append(args, "--devices", joinAddresses(addresses))
	} else {
		args = append(args, "-A")
		plan.Warning = "no exact USBPcap device address match; capturing all devices on the only available USBPcap interface, which can include sibling device traffic"
	}
	plan.Args = args
	return plan, nil
}

func bestInterfaceMatch(interfaces []Interface, target model.DeviceSnapshot) (Interface, []int, []string) {
	var best Interface
	var bestAddresses []int
	var bestReasons []string
	bestScore := -1
	for _, iface := range interfaces {
		addressScore := map[int]int{}
		reasonsByAddress := map[int][]string{}
		for _, device := range iface.Devices {
			score, reasons := matchConfigDevice(device, target)
			if score == 0 || device.Address == 0 {
				continue
			}
			addressScore[device.Address] += score
			reasonsByAddress[device.Address] = append(reasonsByAddress[device.Address], reasons...)
		}
		total := 0
		for _, score := range addressScore {
			total += score
		}
		if total > bestScore {
			best = iface
			bestScore = total
			bestAddresses = sortedAddressKeys(addressScore)
			bestReasons = uniqueReasons(reasonsByAddress, bestAddresses)
		}
	}
	if bestScore <= 0 {
		return Interface{}, nil, nil
	}
	return best, bestAddresses, bestReasons
}

func matchConfigDevice(device Device, target model.DeviceSnapshot) (int, []string) {
	haystack := normalize(device.Display)
	var score int
	var reasons []string
	if target.COMPort != "" && strings.Contains(haystack, normalize(target.COMPort)) {
		score += 100
		reasons = append(reasons, "COM port matched USBPcap device list")
	}
	if target.Serial != "" && strings.Contains(haystack, normalize(target.Serial)) {
		score += 90
		reasons = append(reasons, "serial matched USBPcap device list")
	}
	if target.VID != "" && strings.Contains(haystack, normalize(target.VID)) {
		score += 35
		reasons = append(reasons, "VID matched USBPcap device list")
	}
	if target.PID != "" && strings.Contains(haystack, normalize(target.PID)) {
		score += 35
		reasons = append(reasons, "PID matched USBPcap device list")
	}
	for _, name := range []string{target.FriendlyName, target.Description} {
		if name == "" {
			continue
		}
		needle := normalizeName(name)
		if needle != "" && strings.Contains(haystack, needle) {
			score += 45
			reasons = append(reasons, "device name matched USBPcap device list")
		}
	}
	if target.RelationRole != "" && strings.Contains(haystack, normalize(target.RelationRole)) {
		score += 10
		reasons = append(reasons, "relation role matched USBPcap device list")
	}
	return score, reasons
}

func normalize(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, "("); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return normalize(value)
}

func sortedAddressKeys(scores map[int]int) []int {
	addresses := make([]int, 0, len(scores))
	for address := range scores {
		addresses = append(addresses, address)
	}
	sort.Ints(addresses)
	return addresses
}

func uniqueReasons(reasonsByAddress map[int][]string, addresses []int) []string {
	seen := map[string]bool{}
	var reasons []string
	for _, address := range addresses {
		for _, reason := range reasonsByAddress[address] {
			if seen[reason] {
				continue
			}
			seen[reason] = true
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

func joinAddresses(addresses []int) string {
	parts := make([]string, 0, len(addresses))
	for _, address := range addresses {
		parts = append(parts, strconv.Itoa(address))
	}
	return strings.Join(parts, ",")
}

func countDevices(interfaces []Interface) int {
	count := 0
	for _, iface := range interfaces {
		count += len(iface.Devices)
	}
	return count
}

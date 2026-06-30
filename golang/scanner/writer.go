package scanner

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

// result holds a scanned IP and its mean download latency in milliseconds.
type result struct {
	IP      string
	Latency int // ms
}

// resultStore is a thread-safe store of successful scan results.
type resultStore struct {
	mu      sync.Mutex
	results []result
}

func (s *resultStore) add(ip string, latency int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result{IP: ip, Latency: latency})
}

func (s *resultStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.results)
}

// saveSorted writes results sorted by latency to a .txt file (one "IP latencyMs" per line).
func (s *resultStore) saveSorted(path string) error {
	s.mu.Lock()
	// shallow copy for sorting
	sorted := make([]result, len(s.results))
	copy(sorted, s.results)
	s.mu.Unlock()

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Latency < sorted[j].Latency
	})

	var lines []string
	for _, r := range sorted {
		lines = append(lines, fmt.Sprintf("%s %dms", r.IP, r.Latency))
	}
	data := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(data), 0644)
}

// appendResult appends a single result line to the interim results file.
func appendResult(path string, ip string, latency int) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open results file: %v", err)
		return
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s %dms\n", ip, latency); err != nil {
		log.Printf("Failed to write result: %v", err)
	}
}



// ensureResultsFile creates the interim results file if it doesn't exist.
func ensureResultsFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create results file: %w", err)
	}
	f.Close()
	return nil
}

// globalStore is the module-level store used during a scan run.
var globalStore = &resultStore{}

// resetStore resets the global store for a new run (useful in tests).
func resetStore() {
	globalStore = &resultStore{}
}

// InitResultsFile ensures the output file exists and returns any error.
func InitResultsFile(path string) error {
	return ensureResultsFile(path)
}

// RecordSuccess records a successful IP result: appends it to
// the interim file, and updates the sorted final file.
func RecordSuccess(ip string, latency int, resultsPath, finalPath string) {
	// Append to interim file
	appendResult(resultsPath, ip, latency)

	// Update sorted final file
	globalStore.add(ip, latency)
	if err := globalStore.saveSorted(finalPath); err != nil {
		log.Printf("Failed to update sorted results: %v", err)
	}
}

// SuccessCount returns the number of successful results so far.
func SuccessCount() int {
	return globalStore.count()
}

// SaveFinal writes a final sorted result file.
func SaveFinal(path string) error {
	return globalStore.saveSorted(path)
}

// PrintScannerHelp prints keyboard shortcut hints.
func PrintScannerHelp() {
	fmt.Println("  Controls: [P] Pause  [R] Resume  [S] Skip subnet  [ESC/Ctrl+C] Quit")
}

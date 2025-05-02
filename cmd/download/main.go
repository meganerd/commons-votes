// cmd/download/main.go
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// downloadFile streams a GET from url into outPath.
func downloadFile(url, outPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s failed: %w", outPath, err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// parseParlSession turns "39-1" into (39,1).
func parseParlSession(ps string) (int, int) {
	var parl, sess int
	fmt.Sscanf(ps, "%d-%d", &parl, &sess)
	return parl, sess
}

// downloadSessionSummaries downloads the summary CSV for one session
// and returns all its parsed rows.
func downloadSessionSummaries(code, downloadsDir string) ([][]string, error) {
	url := fmt.Sprintf(
		"https://www.ourcommons.ca/Members/en/votes/csv?parlSession=%s",
		code,
	)
	tmp := filepath.Join(downloadsDir, "summary_"+strings.ReplaceAll(code, "-", "_")+".csv")
	if err := downloadFile(url, tmp); err != nil {
		return nil, err
	}

	f, err := os.Open(tmp)
	if err != nil {
		return nil, fmt.Errorf("open %s failed: %w", tmp, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r.ReadAll()
}

// downloadDetailCSV downloads the per-vote-detail CSV given parl, sess, voteNumber.
func downloadDetailCSV(parl, sess, voteNum, downloadsDir string) error {
	url := fmt.Sprintf(
		"https://www.ourcommons.ca/Members/en/votes/%s/%s/%s/csv",
		parl, sess, voteNum,
	)
	out := filepath.Join(
		downloadsDir,
		fmt.Sprintf("detail_%s_%s_%s.csv", parl, sess, voteNum),
	)
	return downloadFile(url, out)
}

func main() {
	start := flag.String("start", "", "Start Parliament-Session (e.g., 39-1)")
	end := flag.String("end", "", "End Parliament-Session (e.g., 44-1)")
	downloadsDir := flag.String("out", "./downloads", "Directory for CSV downloads")
	flag.Parse()

	if *start == "" || *end == "" {
		fmt.Fprintln(os.Stderr, "Error: must specify --start and --end")
		os.Exit(1)
	}

	if err := os.MkdirAll(*downloadsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", *downloadsDir, err)
		os.Exit(1)
	}

	// Parse the start and end session
	sp, ss := parseParlSession(*start)
	ep, es := parseParlSession(*end)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // up to 5 concurrent downloads

	for p := sp; p <= ep; p++ {
		for s := 1; s <= 5; s++ {
			// skip sessions outside requested range
			if (p == sp && s < ss) || (p == ep && s > es) {
				continue
			}
			parl := strconv.Itoa(p)
			sess := strconv.Itoa(s)
			code := fmt.Sprintf("%s-%s", parl, sess)

			// 1) Download and parse summary
			rows, err := downloadSessionSummaries(code, *downloadsDir)
			if err != nil {
				fmt.Printf("Warning: summary %s failed: %v\n", code, err)
				continue
			}

			// 2) For each vote in the summary, fetch its detail CSV
			for i, row := range rows {
				if i == 0 || len(row) < 4 {
					continue // skip header or malformed line
				}
				voteNum := row[3] // 4th column is VoteNumber
				wg.Add(1)
				go func(parl, sess, voteNum string) {
					defer wg.Done()
					sem <- struct{}{}
					fmt.Printf("Downloading detail %s/%s/%s …\n", parl, sess, voteNum)
					if err := downloadDetailCSV(parl, sess, voteNum, *downloadsDir); err != nil {
						fmt.Printf("  Warning detail %s/%s/%s failed: %v\n", parl, sess, voteNum, err)
					}
					<-sem
				}(parl, sess, voteNum)
			}
		}
	}

	wg.Wait()
}

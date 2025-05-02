// cmd/import/main.go
package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/meganerd/commons-votes/internal/db"
	"github.com/meganerd/commons-votes/internal/model"

	// Import for sql.Row type returned by QueryRow
	_ "database/sql"
)

// Git commit SHA injected at build time
var GitCommit string = "unknown"

func main() {
	// --version support
	for _, arg := range os.Args {
		if arg == "--version" {
			fmt.Println("commons-votes import")
			fmt.Printf("Git Commit: %s\n", GitCommit)
			os.Exit(0)
		}
	}

	dbPath := "./commons.db"
	downloadsDir := "./downloads"

	database, err := db.NewDatabase(dbPath)
	if err != nil {
		panic(err)
	}
	defer database.Close()

	if err := database.Init(); err != nil {
		panic(err)
	}

	// speed‐up PRAGMAs
	database.Exec("PRAGMA synchronous = OFF")
	database.Exec("PRAGMA journal_mode = MEMORY")

	// wrap all imports in one big transaction
	if _, err := database.Exec("BEGIN"); err != nil {
		panic(err)
	}
	if err := importAllVotes(database, downloadsDir); err != nil {
		fmt.Printf("Error importing votes: %v\n", err)
	}
	if _, err := database.Exec("COMMIT"); err != nil {
		panic(err)
	}

	summarizeSkippedVotes()
	fmt.Println("\033[1;32m✅ Bootstrap complete! Database is ready.\033[0m")
}

// readCSV is a helper to read CSV files
func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord, r.LazyQuotes = -1, true
	return r.ReadAll()
}

func importAllVotes(db *db.Database, dir string) error {
	// caches
	memberCache := make(map[string]int)
	billCache := make(map[string]int) // key = "39-1_219" → billID

	// 1) Read each summary_N_M.csv to populate bills
	sumFiles, _ := filepath.Glob(filepath.Join(dir, "summary_*.csv"))
	for _, sumPath := range sumFiles {
		rows, err := readCSV(sumPath)
		if err != nil {
			continue
		}
		// Extract session from filename: summary_39_1.csv -> 39_1
		base := filepath.Base(sumPath)
		session := strings.TrimSuffix(strings.TrimPrefix(base, "summary_"), ".csv")
		for i, rec := range rows {
			if i == 0 || len(rec) < 10 {
				continue
			}
			voteNum, billNum, desc := rec[3], rec[9], rec[4]

			// Generate a URL for the bill if it has a number
			var fullTextURL string
			if billNum != "" {
				// Extract parliament and session from the session string (e.g., "39_1" -> parliament 39, session 1)
				parts := strings.Split(session, "_")
				if len(parts) == 2 {
					parliament, sessionNum := parts[0], parts[1]
					// Format: C-31 -> c-31 (lowercase)
					billNumForURL := strings.ToLower(billNum)
					fullTextURL = fmt.Sprintf("https://www.parl.ca/legisinfo/en/bill/%s-%s/%s", parliament, sessionNum, billNumForURL)
				}
			}

			// InsertBill now returns the real ID
			id, err := db.InsertBill(model.Bill{
				Number:      billNum,
				Description: desc,
				FullTextURL: fullTextURL,
			})
			if err != nil {
				fmt.Printf("  Warning inserting bill %s: %v\n", billNum, err)
				continue
			}
			billCache[session+"_"+voteNum] = id
		}
	}

	// 2) Now import all detail CSVs, passing in the correct billID
	totalImp, totalSkip := 0, 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, _ error) error {
		name := d.Name()
		if strings.HasPrefix(name, "detail_") && strings.HasSuffix(name, ".csv") {
			// parse session & voteNum from filename: detail_39_1_219.csv
			// Extract session and voteNum from filename: detail_39_1_100.csv -> 39_1, 100
			parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "detail_"), ".csv"), "_")
			session, voteNum := parts[0]+"_"+parts[1], parts[2]
			billID := billCache[session+"_"+voteNum]

			imp, skip, _ := importVotesWithBill(db, path, memberCache, billID)
			totalImp += imp
			totalSkip += skip
		}
		return nil
	})
	fmt.Printf("\nTotal Imported Votes: %d   Total Skipped Votes: %d\n", totalImp, totalSkip)
	return nil
}

func importVotesWithBill(database *db.Database, path string, memberCache map[string]int, billID int) (int, int, error) {
	// 1) Open the CSV file
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	// 2) Read all records
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // allow variable fields
	r.LazyQuotes = true    // tolerate sloppy quotes
	records, err := r.ReadAll()
	if err != nil {
		return 0, 0, err
	}

	imported, skipped := 0, 0

	for i, rec := range records {
		if i == 0 {
			continue // skip header
		}
		if len(rec) < 3 {
			skipped++
			continue
		}

		// detail CSV columns: Name, Party, Result
		name, party, result := rec[0], rec[1], rec[2]

		// --- member lookup/insert ---
		memberID, ok := memberCache[name]
		if !ok {
			m := model.Member{Name: name, Party: party}
			if err := database.InsertMember(m); err != nil {
				skipped++
				continue
			}
			// fetch the new ID via helper
			err := database.QueryRow(
				"SELECT id FROM members WHERE name = ?", name,
			).Scan(&memberID)
			if err != nil {
				skipped++
				continue
			}
			memberCache[name] = memberID
		}

		// --- vote insert ---
		vote := model.Vote{
			BillID:   billID,
			MemberID: memberID,
			Result:   result,
			VoteDate: "", // detail CSV has no date
		}

		if err := database.InsertVote(vote); err != nil {
			skipped++
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d votes. Skipped %d from %s\n", imported, skipped, path)
	return imported, skipped, nil
}

func summarizeSkippedVotes() {
	f, err := os.Open("skipped_votes.log")
	if err != nil {
		fmt.Println("No skipped votes to summarize.")
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	total, miss, dbErr := 0, 0, 0
	for scanner.Scan() {
		total++
		line := scanner.Text()
		if strings.Contains(line, "missing fields") {
			miss++
		} else if strings.Contains(line, "DB error") {
			dbErr++
		}
	}
	fmt.Println("\nSkipped Votes Summary:")
	fmt.Printf("  Total Skipped: %d\n", total)
	fmt.Printf("    Missing Fields: %d\n", miss)
	fmt.Printf("    DB Insert Errors: %d\n", dbErr)
}

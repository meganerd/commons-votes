package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/meganerd/commons-votes/internal/db"
	"github.com/meganerd/commons-votes/internal/model"

	_ "modernc.org/sqlite" // Import SQLite driver
)

// stripParentheses removes any " (…)" suffix
func stripParentheses(s string) string {
	if idx := strings.Index(s, " ("); idx >= 0 {
		return s[:idx]
	}
	return s
}

var GitCommit string = "unknown"

func main() {
	// --version flag
	for _, arg := range os.Args {
		if arg == "--version" {
			fmt.Println("commons-votes query")
			fmt.Printf("Git Commit: %s\n", GitCommit)
			os.Exit(0)
		}
	}

	namePtr := flag.String("name", "", "MP name or riding (fuzzy)")
	idsPtr := flag.String("id", "", "MP ID(s) (comma-separated list, e.g. 123,456)")
	flag.Parse()

	if *namePtr == "" && *idsPtr == "" {
		log.Fatal("Please specify either --name (e.g. --name \"Yellowknife\") or --id (e.g. --id 123,456)")
	}

	// Open DB
	database, err := db.NewDatabase("./commons.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	var matches []model.Member

	if *idsPtr != "" {
		// Direct lookup by ID(s)
		idStrings := strings.Split(*idsPtr, ",")
		for _, idStr := range idStrings {
			idStr = strings.TrimSpace(idStr)
			id, err := strconv.Atoi(idStr)
			if err != nil {
				log.Fatalf("Invalid ID format: %s. Please use comma-separated integers.", idStr)
			}

			row := database.QueryRow("SELECT id, name, party FROM members WHERE id = ?", id)
			var m model.Member
			err = row.Scan(&m.ID, &m.Name, &m.Party)
			if err != nil {
				fmt.Printf("Warning: No member found with ID %d\n", id)
				continue
			}
			matches = append(matches, m)
		}

		if len(matches) == 0 {
			fmt.Println("No members found with the specified ID(s).")
			return
		}
	} else {
		// Fuzzy search by name
		// Load all members
		rows, err := database.Query("SELECT id, name, party FROM members")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		var members []model.Member
		for rows.Next() {
			var m model.Member
			if err := rows.Scan(&m.ID, &m.Name, &m.Party); err != nil {
				log.Fatal(err)
			}
			members = append(members, m)
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}

		// Fuzzy‐match user input against "Name (Riding)"
		input := strings.ToLower(*namePtr)
		for _, m := range members {
			if fuzzy.Match(input, strings.ToLower(m.Name)) {
				matches = append(matches, m)
			}
		}

		if len(matches) == 0 {
			fmt.Println("No members matched your search.")
			return
		}
	}

	// Build groups by base name
	groups := make(map[string][]model.Member)
	for _, m := range matches {
		base := stripParentheses(m.Name)
		groups[base] = append(groups[base], m)
	}

	// Iterate groups in sorted order
	bases := make([]string, 0, len(groups))
	for base := range groups {
		bases = append(bases, base)
	}
	sort.Strings(bases)

	for _, base := range bases {
		fmt.Printf("\n— %s —\n", base)
		for _, m := range groups[base] {
			fmt.Printf("  ID=%d: %s  [%s]\n", m.ID, m.Name, m.Party)
		}
	}

	// If only one base, drill into that. Otherwise you could
	// prompt the user to choose a base or automatically pick the first.
	if len(bases) == 1 {
		selBase := bases[0]
		for _, m := range groups[selBase] {
			fmt.Printf("\nVotes for %s:\n", m.Name)
			voteRows, err := database.Query(`
				SELECT v.vote_date, v.result, COALESCE(b.number, ''), COALESCE(b.description, 'Unknown bill'), COALESCE(b.full_text_url, '')
				FROM votes v
				LEFT JOIN bills b ON v.bill_id = b.id
				WHERE v.member_id = ?
				ORDER BY v.vote_date DESC
			`, m.ID)
			if err != nil {
				fmt.Printf("  error querying votes: %v\n", err)
				continue
			}
			defer voteRows.Close()

			found := false
			for voteRows.Next() {
				found = true
				var date, result, billNum, billDesc, fullTextURL string
				voteRows.Scan(&date, &result, &billNum, &billDesc, &fullTextURL)
				if fullTextURL != "" {
					fmt.Printf("  %s → %s  [%s] %s [Full Text: %s]\n", date, result, billNum, billDesc, fullTextURL)
				} else {
					fmt.Printf("  %s → %s  [%s] %s\n", date, result, billNum, billDesc)
				}
			}
			if !found {
				fmt.Println("  (no votes found for this MP)")
			}
		}
	} else {
		fmt.Println("\nMultiple matching MPs—please refine your --name or pick an ID above.")
	}
}

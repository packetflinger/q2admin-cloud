package main

import (
    "encoding/csv"
    "fmt"
    "io"
    "log"
    "os"
    "strings"
)

type Ban struct {
    address     string
    condition   string
    value       string
    description string
}

var globalbans []Ban

/**
 * Parse ban files loading records into memory
 */
func LoadGlobalBans() {
    banfile := "bans/global.csv"
    bandata, err := os.ReadFile(banfile)
    if err != nil {
        log.Fatal(err)
    }

    r := csv.NewReader(strings.NewReader(string(bandata)))
    for {
        r.Comment = '#'
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

        ban := Ban{
            address: record[0],
            description: record[2],
        }

        globalbans = append(globalbans, ban)
	}

    fmt.Println(globalbans)
}

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

var Version = "development" // default value

type Config struct {
	DB struct {
		Type     string `json:"type"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"db"`
}

func main() {
	// Define command-line flag
	var showVersion bool
	configFile := flag.String("config", "config.json", "path to config file")
	flag.BoolVar(&showVersion, "version", false, "show version infomration and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(1)
	}

	// Read config
	viper.SetConfigFile(*configFile)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	// Create DB connection string
	var db *sql.DB
	var err error
	if config.DB.Type == "postgresql" {
		db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.DB.Host, config.DB.Port, config.DB.User, config.DB.Password, config.DB.Name))
	} else if config.DB.Type == "mysql" {
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name))
	} else {
		log.Fatalf("Unsupported DB type: %s", config.DB.Type)
	}

	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Perform the query
	query := ""
	if config.DB.Type == "postgresql" {
		query = "SELECT props FROM sessions WHERE props != '{}'"
	} else if config.DB.Type == "mysql" {
		query = "SELECT props FROM Sessions WHERE JSON_LENGTH(props) > 0"
	}

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Error executing query: %v", err)
	}
	defer rows.Close()

	type Props struct {
		Browser string `json:"browser"`
	}

	versionCount := make(map[string]int)

	for rows.Next() {
		var props string
		if err := rows.Scan(&props); err != nil {
			log.Fatalf("Error scanning row: %v", err)
		}

		var propData Props
		if err := json.Unmarshal([]byte(props), &propData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			continue
		}

		if strings.Contains(propData.Browser, "Desktop App") {
			parts := strings.Split(propData.Browser, "/")
			if len(parts) == 2 {
				version := parts[1]
				versionCount[version]++
			}
		}
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating over rows: %v", err)
	}

	// Print the tally
	fmt.Printf("Mattermost Desktop App Versions Found:\n")
	for version, count := range versionCount {
		fmt.Printf("    %s - %d\n", version, count)
	}
}

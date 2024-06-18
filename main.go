package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

type Props struct {
	Browser  string `json:"browser"`
	OS       string `json:"os"`
	IsMobile string `json:"isMobile"`
	DeviceID string `json:"deviceid"`
}

type VersionInfo struct {
	OS    string
	Count int
}

type VersionCount map[string][]VersionInfo

var debugMode bool = false

// LogLevel is used to refer to the type of message that will be written using the logging code.
type LogLevel string

const (
	debugLevel   LogLevel = "DEBUG"
	infoLevel    LogLevel = "INFO"
	warningLevel LogLevel = "WARNING"
	errorLevel   LogLevel = "ERROR"
)

// Logging functions

// LogMessage logs a formatted message to stdout or stderr
func LogMessage(level LogLevel, message string) {
	if level == errorLevel {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(os.Stdout)
	}
	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("[%s] %s\n", level, message)
}

// DebugPrint allows us to add debug messages into our code, which are only printed if we're running in debug more.
// Note that the command line parameter '-debug' can be used to enable this at runtime.
func DebugPrint(message string) {
	if debugMode {
		LogMessage(debugLevel, message)
	}
}

func loadConfig(configFile string) (*Config, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		errMsg := fmt.Sprintf("Error reading config file, %s", err)
		LogMessage(errorLevel, errMsg)
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		errMsg := fmt.Sprintf("Unable to decode into struct, %v", err)
		LogMessage(errorLevel, errMsg)
		return nil, err
	}

	return &config, nil
}

func connectDatabase(config *Config) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if config.DB.Type == "postgresql" {
		db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.DB.Host, config.DB.Port, config.DB.User, config.DB.Password, config.DB.Name))
	} else if config.DB.Type == "mysql" {
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name))
	} else {
		errMsg := fmt.Sprintf("Unsupported DB type: %s", config.DB.Type)
		LogMessage(errorLevel, errMsg)
		return nil, err
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error opening database: %v", err)
		LogMessage(errorLevel, errMsg)
		return nil, err
	}

	return db, nil
}

func processDatabase(db *sql.DB, dbType string) (VersionCount, VersionCount, error) {

	// We need the current epoch to ensure we only retrieve sessions that are still active
	currentEpochMillis := time.Now().UnixMilli()

	query := ""
	if dbType == "postgresql" {
		query = fmt.Sprintf("SELECT props, deviceid, expiresat FROM sessions WHERE props != '{}' AND (expiresat > %d OR expiresat = 0)", currentEpochMillis)
	} else if dbType == "mysql" {
		query = fmt.Sprintf("SELECT props, DeviceId, ExpiresAt FROM Sessions WHERE JSON_LENGTH(props) > 0 AND (ExpiresAt > %d OR ExpiresAt = 0)", currentEpochMillis)
	}

	rows, err := db.Query(query)
	if err != nil {
		errMsg := fmt.Sprintf("Error executing query: %v", err)
		LogMessage(errorLevel, errMsg)
		return nil, nil, err
	}
	defer rows.Close()

	desktopVersionCount := make(VersionCount)
	mobileVersionCount := make(VersionCount)

	for rows.Next() {
		var props, deviceID string
		var expiresAt int64
		if dbType == "postgresql" {
			if err := rows.Scan(&props, &deviceID, &expiresAt); err != nil {
				errMsg := fmt.Sprintf("Error scanning PostgreSQL row: %v", err)
				LogMessage(errorLevel, errMsg)
				return nil, nil, err
			}
		} else if dbType == "mysql" {
			if err := rows.Scan(&props, &deviceID, &expiresAt); err != nil {
				errMsg := fmt.Sprintf("Error scanning MySQL row: %v", err)
				LogMessage(errorLevel, errMsg)
				return nil, nil, err
			}
		}

		var propData Props
		if err := json.Unmarshal([]byte(props), &propData); err != nil {
			errMsg := fmt.Sprintf("Error unmarshalling JSON: %v", err)
			LogMessage(warningLevel, errMsg)
			continue
		}
		propData.DeviceID = deviceID

		if propData.IsMobile == "true" || deviceID != "" || propData.OS == "Android" || propData.OS == "iOS" {
			parts := strings.Split(propData.Browser, "/")
			if len(parts) == 2 {
				versionParts := strings.Split(parts[1], "+")
				version := versionParts[0]
				if version == "0.0" {
					errMsg := fmt.Sprintf("Unrecognised entry - Device ID: %s, JSON Session: %s", deviceID, props)
					LogMessage(warningLevel, errMsg)
				}
				if mobileVersionCount[version] == nil {
					mobileVersionCount[version] = make([]VersionInfo, 0)
				}
				mobileVersionCount[version] = append(mobileVersionCount[version], VersionInfo{OS: propData.OS, Count: 1})
			}
		} else if strings.Contains(propData.Browser, "Desktop App") {
			parts := strings.Split(propData.Browser, "/")
			if len(parts) == 2 {
				version := parts[1]
				if version == "0.0" {
					debugMessage := fmt.Sprintf("Troubleshooting: %s", props)
					DebugPrint(debugMessage)
					continue
				}
				if desktopVersionCount[version] == nil {
					desktopVersionCount[version] = make([]VersionInfo, 0)
				}
				desktopVersionCount[version] = append(desktopVersionCount[version], VersionInfo{OS: propData.OS, Count: 1})
			}
		}
	}

	if err := rows.Err(); err != nil {
		errMsg := fmt.Sprintf("Error iterating over rows: %v", err)
		LogMessage(errorLevel, errMsg)
		return nil, nil, err
	}

	aggregateCounts(desktopVersionCount)
	aggregateCounts(mobileVersionCount)

	return desktopVersionCount, mobileVersionCount, nil
}

func aggregateCounts(versionCount VersionCount) {
	for version, infos := range versionCount {
		osCount := make(map[string]int)
		for _, info := range infos {
			osCount[info.OS] += info.Count
		}

		versionCount[version] = nil
		for os, count := range osCount {
			versionCount[version] = append(versionCount[version], VersionInfo{OS: os, Count: count})
		}
	}
}

func printResults(desktopVersionCount, mobileVersionCount VersionCount) {
	hasDesktopApps := len(desktopVersionCount) > 0
	hasMobileApps := len(mobileVersionCount) > 0

	totalDesktopClients := len(desktopVersionCount)
	totalMobileClients := len(mobileVersionCount)
	totalActiveClients := totalDesktopClients + totalMobileClients

	if !hasDesktopApps && !hasMobileApps {
		fmt.Println("No Mattermost Apps Found")
	} else {
		if hasDesktopApps {
			fmt.Println("Mattermost Desktop App Versions Found:")
			for version, infos := range desktopVersionCount {
				for _, info := range infos {
					fmt.Printf("  %s (%s) - %d\n", version, info.OS, info.Count)
				}
			}
			fmt.Printf("\nTotal Active Desktop Clients: %d\n", totalDesktopClients)
		} else {
			fmt.Println("No Mattermost Desktop Apps Found")
		}

		if hasMobileApps {
			fmt.Println("\nMattermost Mobile App Versions Found:")
			for version, infos := range mobileVersionCount {
				for _, info := range infos {
					fmt.Printf("  %s (%s) - %d\n", version, info.OS, info.Count)
				}
			}
			fmt.Printf("\nTotal Active Mobile Clients: %d\n", totalMobileClients)
		} else {
			fmt.Println("No Mattermost Mobile Apps Found")
		}

		fmt.Printf("\nTotal Active Clients: %d\n", totalActiveClients)
	}
}

func main() {
	// Define command-line flag
	var showVersion bool
	configFile := flag.String("config", "config.json", "path to config file")
	flag.BoolVar(&showVersion, "version", false, "show version infomration and exit")
	flag.BoolVar(&debugMode, "debug", false, "run the utility in debug mode for additional output")
	flag.Parse()

	if showVersion {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(1)
	}

	config, cfgErr := loadConfig(*configFile)
	if cfgErr != nil {
		LogMessage(errorLevel, "Failed to process config file")
		os.Exit(2)
	}
	db, dbErr := connectDatabase(config)
	if dbErr != nil {
		LogMessage(errorLevel, "Failed to connect to database")
		os.Exit(3)
	}
	defer db.Close()

	desktopVersionCount, mobileVersionCount, processErr := processDatabase(db, config.DB.Type)
	if processErr != nil {
		LogMessage(errorLevel, "Error processing database")
		os.Exit(4)
	}

	printResults(desktopVersionCount, mobileVersionCount)
}

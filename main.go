package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

var Version = "development" // default value

var defaultOutputFile = "users.csv"

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

func splitVersion(version string) (int, int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}

	return major, minor, patch, nil
}

func isOlderOrEqual(version, lookupVersion string) (bool, error) {
	vMajor, vMinor, vPatch, err := splitVersion(version)
	if err != nil {
		return false, err
	}

	lvMajor, lvMinor, lvPatch, err := splitVersion(lookupVersion)
	if err != nil {
		return false, err
	}

	if vMajor < lvMajor {
		return true, nil
	}
	if vMajor > lvMajor {
		return false, nil
	}

	// If major versions are equal, compare minor versions
	if vMinor < lvMinor {
		return true, nil
	}
	if vMinor > lvMinor {
		return false, nil
	}

	// If minor versions are equal, compare patch versions
	return vPatch <= lvPatch, nil
}

func doLookup(db *sql.DB, dbType string, outputFilename string, lookupVersion string) error {

	DebugPrint("Running doLookup.  Writing output to: " + outputFilename + " - Processing desktop version prior to " + lookupVersion)

	// Create the output file
	file, err := os.Create(outputFilename)
	if err != nil {
		LogMessage(errorLevel, "Failed to create CSV file: "+err.Error())
		return err
	}
	defer file.Close()

	// Prepare the CSv writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write the CSV header row
	header := []string{"Version", "OS", "Username", "Email", "First Name", "Last Name"}
	if err := writer.Write(header); err != nil {
		LogMessage(errorLevel, "Failed to write header row to CSV: "+err.Error())
		return err
	}

	// We need the current epoch to ensure we only retrieve sessions that are still active
	currentEpochMillis := time.Now().UnixMilli()

	query := ""
	if dbType == "postgresql" {
		query = fmt.Sprintf("SELECT userid, props, deviceid, expiresat FROM sessions WHERE props != '{}' AND (expiresat > %d OR expiresat = 0)", currentEpochMillis)
	} else if dbType == "mysql" {
		query = fmt.Sprintf("SELECT UserId, Props, DeviceId, ExpiresAt FROM Sessions WHERE JSON_LENGTH(props) > 0 AND (ExpiresAt > %d OR ExpiresAt = 0)", currentEpochMillis)
	}

	rows, err := db.Query(query)
	if err != nil {
		errMsg := fmt.Sprintf("Error executing query: %v", err)
		LogMessage(errorLevel, errMsg)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var props, deviceID string
		var expiresAt int64
		var userID string
		if dbType == "postgresql" {
			if err := rows.Scan(&userID, &props, &deviceID, &expiresAt); err != nil {
				errMsg := fmt.Sprintf("Error scanning PostgreSQL row: %v", err)
				LogMessage(errorLevel, errMsg)
				return err
			}
		} else if dbType == "mysql" {
			if err := rows.Scan(&userID, &props, &deviceID, &expiresAt); err != nil {
				errMsg := fmt.Sprintf("Error scanning MySQL row: %v", err)
				LogMessage(errorLevel, errMsg)
				return err
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
			DebugPrint("Mobile device.  Skipping for lookup.")
		} else if strings.Contains(propData.Browser, "Desktop App") {
			version := ""
			processRow := false
			var err error
			parts := strings.Split(propData.Browser, "/")
			if len(parts) == 2 {
				version = parts[1]
				if version == "0.0" {
					debugMessage := fmt.Sprintf("Troubleshooting: %s", props)
					DebugPrint(debugMessage)
					continue
				}

				processRow, err = isOlderOrEqual(version, lookupVersion)
				if err != nil {
					LogMessage(warningLevel, "Unable to parse version string: "+version)
					processRow = true
				}
			}

			if processRow {
				userQuery := ""
				if dbType == "postgresql" {
					userQuery = fmt.Sprintf("SELECT username, email, firstname, lastname FROM users WHERE id = '%s'", userID)
				} else if dbType == "mysql" {
					userQuery = fmt.Sprintf("SELECT Username, Email, FirstName, LastName FROM Users WHERE Id = '%s'", userID)
				}

				userRows, err := db.Query(userQuery)
				if err != nil {
					errMsg := fmt.Sprintf("Error executing query: %v", err)
					LogMessage(errorLevel, errMsg)
					return err
				}
				defer userRows.Close()

				for userRows.Next() {
					var username, email, firstname, lastname string
					if dbType == "postgresql" {
						if err := userRows.Scan(&username, &email, &firstname, &lastname); err != nil {
							errMsg := fmt.Sprintf("Error scanning PostgreSQL row: %v", err)
							LogMessage(errorLevel, errMsg)
							return err
						}
					} else if dbType == "mysql" {
						if err := userRows.Scan(&username, &email, &firstname, &lastname); err != nil {
							errMsg := fmt.Sprintf("Error scanning MySQL row: %v", err)
							LogMessage(errorLevel, errMsg)
							return err
						}
					}

					csvRecord := []string{version, propData.OS, username, email, firstname, lastname}

					// Write the record
					if err := writer.Write(csvRecord); err != nil {
						warningMessage := fmt.Sprintf("Failed to write record to CSV! Version: %s, OS: %s, Usermame: %s, Email: %s, Name: %s %s",
							version,
							propData.OS,
							username,
							email,
							firstname,
							lastname)
						LogMessage(warningLevel, warningMessage)
					}
				}
			}
		}
	}

	return nil
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

	totalDesktopClients := 0
	totalMobileClients := 0

	for _, desktopVersionInfos := range desktopVersionCount {
		for _, desktopVersionInfo := range desktopVersionInfos {
			totalDesktopClients += desktopVersionInfo.Count
		}
	}
	for _, mobileVersionInfos := range mobileVersionCount {
		for _, mobileVersionInfo := range mobileVersionInfos {
			totalMobileClients += mobileVersionInfo.Count
		}
	}

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
	var lookupMode bool
	var lookupVersion string
	var outputFile string
	configFile := flag.String("config", "config.json", "path to config file")
	flag.BoolVar(&lookupMode, "lookup", false, "lookup desktop users prior to an existing version")
	flag.StringVar(&lookupVersion, "ver", "", "[required for lookup] user with desktop clients of this version and older will be returned")
	flag.StringVar(&outputFile, "outfile", defaultOutputFile, "[optional] Specify an alternative output CSV filename when using lookup mode.  Default:"+defaultOutputFile)
	flag.BoolVar(&showVersion, "version", false, "show version infomration and exit")
	flag.BoolVar(&debugMode, "debug", false, "run the utility in debug mode for additional output")
	flag.Parse()

	if showVersion {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(1)
	}

	if lookupMode {
		if lookupVersion == "" {
			LogMessage(errorLevel, "A desktop client version is required for lookup mode")
			flag.Usage()
			os.Exit(1)
		}
		LogMessage(infoLevel, "Running in lookup mode, for desktop version v"+lookupVersion+" and earlier.  Writing results to: "+outputFile)
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

	if lookupMode {
		DebugPrint("Staring lookup")
		lookupErr := doLookup(db, config.DB.Type, outputFile, lookupVersion)
		if lookupErr != nil {
			LogMessage(errorLevel, "Error processing lookup")
			os.Exit(10)
		}
	} else {
		desktopVersionCount, mobileVersionCount, processErr := processDatabase(db, config.DB.Type)
		if processErr != nil {
			LogMessage(errorLevel, "Error processing database")
			os.Exit(4)
		}

		printResults(desktopVersionCount, mobileVersionCount)
	}
}

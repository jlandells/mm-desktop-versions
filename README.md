# Mattermost Browser Version Tally Utility

This Go utility connects to a Mattermost database, extracts session data from a JSON field, and counts the number of sessions running different versions of a desktop or mobile application. The configuration is read from a JSON file using Viper, and you can specify a different config file using a command-line flag.

## Features

- Reads database configuration from a JSON file.
- Connects to either PostgreSQL or MySQL databases.
- Queries session data and extracts JSON properties.
- Filters and tallies desktop and mobile app versions for each OS from session data.
- Outputs the tally of different app versions.

## Configuration

The configuration file should be in JSON format. Below is an example `config.json` file:

```json
{
    "db": {
        "type": "postgresql",
        "host": "localhost",
        "port": 5432,
        "name": "your_db_name",
        "user": "your_db_user",
        "password": "your_db_password"
    }
}
```

> [!IMPORTANT]
> The `type` **must** be either `postgresql` or `mysql`.  No other database types are supported.

## Usage

### Running the Utility
- Ensure you have the configuration file (`config.json`) in the same directory as the executable or specify the path to the configuration file using the `-config` flag.
- Run the utility with the default configuration file (`config.json`):
```sh
./mm-desktop-versions-<arch>
```
- To specify a different configuration file:
```sh
./mm-desktop-versions-<arch> -config=custom_config.json
```
- To obtain the version information from this utility:
```sh
./mm-desktop-versions-<arch> -version
```

> [!NOTE]
> Replace `<arch>` with the appropriate architecture of your executable (e.g., `amd64`, `arm64`). This `README.md` file assumes that the users will be using a precompiled binary, simplifying the usage instructions and removing the need for them to install Go and any dependencies.

### Sample Output

The output will be a tally of different versions of the desktop or mobile application found in the session data:
```
Mattermost Desktop App Versions Found:
  5.3.0 (Windows) - 23
  5.4.0 (Windows) - 46
  5.5.0 (Windows) - 67
  5.5.0 (Mac OS) - 20
  5.5.3 (Windows) - 89
  5.5.3 (Mac OS) - 24
  5.5.3 (Linux) - 3
  5.8.0 (Windows) - 5
  5.8.0 (Mac OS) - 6

Mattermost Mobile App Versions Found:
  2.13.4 (iOS) - 234
  2.13.4 (Android) - 43
  2.17.0 (iOS) - 64
```

## Installation

- Download the appropriate executable for your architecture (`mm-desktop-versions-<arch>`).
- Place the executable in your desired directory.
- For Mac and Linux systems, ensure the file is executable:
```sh
chmod +x ./mm-desktop-versions-<arch>
```
- Create your `config.json` file based on the example provided above.
- Run the utility:
```sh
./mm-desktop-versions-<arch>
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your improvements.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
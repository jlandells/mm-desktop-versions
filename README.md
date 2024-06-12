# Session Browser Version Tally Utility

This Go utility connects to a Mattermost database, extracts session data from a JSON field, and counts the number of sessions running different versions of a desktop application. The configuration is read from a JSON file using Viper, and you can specify a different config file using a command-line flag.

## Features

- Reads database configuration from a JSON file.
- Connects to either PostgreSQL or MySQL databases.
- Queries session data and extracts JSON properties.
- Filters and tallies desktop app versions from session data.
- Outputs the tally of different desktop app versions.

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

The output will be a tally of different versions of the desktop application found in the session data:
```
Mattermost Desktop App Versions Found:
    5.3.0 - 23
    5.4.0 - 46
    5.5.0 - 87
    5.5.3 - 129
    5.8.0 - 5
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
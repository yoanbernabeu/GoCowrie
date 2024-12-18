
# GoCowrie CLI - Experimental

GoCowrie is an interactive CLI tool designed to help you quickly and efficiently navigate [Cowrie honeypot](https://github.com/cowrie/cowrie) JSON logs. With two screens for navigation, you can easily browse source IPs and drill down into detailed event lists.

## Features

- **Main Screen**:
  - List of unique source IPs.
  - First and last event timestamps.
  - Indication of login success.
- **Detail Screen**:
  - Event list for the selected IP.
  - Timestamp, event ID, username/password, input command, and message columns.
  - **Replay TTY Log Command**:
    - On events of type `cowrie.log.closed`, press Enter to generate and display a replay command.
    - The command can be copied or directly used in the Docker version's bash for replaying the session.

## Installation

To install GoCowrie, run the following command:

```bash
curl -sSL https://raw.githubusercontent.com/yoanbernabeu/GoCowrie/main/install.sh | bash
```

## How to Use

1. **Run the CLI**:
   ```bash
   go-cowrie /path/to/cowrie.json
   ```
   Replace `/path/to/cowrie.json` with the actual path to your Cowrie logs file.

2. **Main Screen**:
   - On startup, you will see a table listing unique source IPs.
   - For each IP, you can see:
     - **First Event Timestamp**
     - **Last Event Timestamp**
     - **Login Success?** (Indicates if a `cowrie.login.success` event occurred)
   
   **Keyboard Controls**:
   - **Up/Down Arrows**: Move the selection between different IP rows.
   - **Enter**: Open the detail screen for the currently selected IP.
   - **Esc**: Exit the application.

3. **Detail Screen**:
   - After pressing Enter on the selected IP, you will see a new table showing all events related to that IP.
   - Columns include:
     - Timestamp
     - Event ID
     - Username/Password (if any)
     - Input Command (if any)
     - Message
   - For events of type `cowrie.log.closed`:
     - Select the event and press Enter to generate a replay command.
     - The generated command can be used directly in the bash shell of the Cowrie Docker container to replay the TTY session.

   **Keyboard Controls**:
   - **Up/Down/Left/Right Arrows**: Scroll through the events if the list is long.
   - **Enter on `cowrie.log.closed`**: Generate the TTY replay command.
   - **Esc**: Return to the main screen.

4. **Exiting the CLI**:
   - To quit, press **Esc** from the main screen.

## Tips

- Ensure that your Cowrie log file is in JSON format, one event per line.
- Use arrow keys to navigate quickly between rows and columns.
- If the event lists are very long, utilize the scroll functionality by pressing arrow keys in the detail screen.
- Pressing **Esc** from the detail screen takes you back to the main screen without losing your place.
- When using the Docker version, the generated TTY replay command can be run directly in the container's bash shell.

## Build from Source

To build GoCowrie from source, you need to have Go installed on your system. Then, run the following commands:

```bash
git clone git@github.com:yoanbernabeu/GoCowrie.git
cd GoCowrie
go build -o go-cowrie
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

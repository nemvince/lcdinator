# LCDinator

LCDinator is a Go application designed to display real-time system and network information on the built-in LCD screen of CheckPoint 4800 (or similar) firewalls. It provides a simple, interactive interface for monitoring system status, network interfaces, and running services directly from the device's display.

## Features

- **System Information:** View CPU usage, memory usage, disk usage, and system uptime.
- **Network Information:** Display network interfaces, IP addresses, link status, and bandwidth usage.
- **Service Manager:** Scroll through running services, and perform actions such as stop or restart.
- **Menu:** Options for system shutdown and reboot, with confirmation dialogs.
- **About Screen:** Project and version information.

## Usage

2. **Build and run** the application on a compatible Linux system:
   ```bash
   go build -o lcdinator
   ./lcdinator /dev/ttyS1
   ```
   Replace `/dev/ttyS1` with the appropriate serial device if needed.
   A systemd service is on the roadmap, I promise.

3. **Navigate** the interface using the device's hardware buttons:
   - Up/Down: Scroll through lists and menu items.
   - Left/Right: Trigger service actions (stop/restart).
   - Enter: Confirm actions or dialogs.
   - Esc: Cancel dialogs or return to previous screens.

## Building

Ensure you have Go installed (version 1.18 or newer recommended).

```bash
go build
```

## Project Structure

- `main.go` — Application entry point and serial communication.
- `screens.go` — UI screens and navigation logic.
- `sysinfo.go` — System and network information gathering.
- `keyhandler.go` — Key/button handling.
- `icon.go`, `icons.go` — Icon drawing utilities.

## License

no.
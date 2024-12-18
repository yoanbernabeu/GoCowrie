package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CowrieEvent represents a single Cowrie event stored as a map
type CowrieEvent map[string]interface{}

type IPInfo struct {
	IP              string
	FirstTimestamp  string
	LastTimestamp   string
	HasLoginSuccess bool
	Events          []CowrieEvent
}

func parseTimestamp(ts string) time.Time {
	// Cowrie timestamps are often in the format "2024-12-17T14:38:20.918891Z"
	// Try parsing them using RFC3339Nano
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{} // Return zero time on parse failure
	}
	return t
}

func extractTTYLogCommand(message string) string {
	// Remove "Closing TTY Log: " and " after ... seconds"
	cleaned := strings.TrimPrefix(message, "Closing TTY Log: ")
	idx := strings.LastIndex(cleaned, " after ")
	if idx != -1 {
		cleaned = cleaned[:idx]
	}
	return fmt.Sprintf("/cowrie/cowrie-git/bin/playlog /cowrie/cowrie-git/%s", cleaned)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.json>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	eventsByIP := make(map[string][]CowrieEvent)

	// Parse events
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var evt CowrieEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
			continue
		}

		srcIP, _ := evt["src_ip"].(string)
		if srcIP == "" {
			srcIP = "UNKNOWN_IP"
		}
		eventsByIP[srcIP] = append(eventsByIP[srcIP], evt)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Group events by IP
	var ipInfos []IPInfo
	for ip, evts := range eventsByIP {
		var firstTime, lastTime time.Time
		hasLoginSuccess := false

		sort.Slice(evts, func(i, j int) bool {
			t1 := parseTimestamp(fmt.Sprint(evts[i]["timestamp"]))
			t2 := parseTimestamp(fmt.Sprint(evts[j]["timestamp"]))
			return t1.Before(t2)
		})

		if len(evts) > 0 {
			firstTime = parseTimestamp(fmt.Sprint(evts[0]["timestamp"]))
			lastTime = parseTimestamp(fmt.Sprint(evts[len(evts)-1]["timestamp"]))
		}

		for _, e := range evts {
			if fmt.Sprint(e["eventid"]) == "cowrie.login.success" {
				hasLoginSuccess = true
				break
			}
		}

		ipInfos = append(ipInfos, IPInfo{
			IP:              ip,
			FirstTimestamp:  firstTime.Format(time.RFC3339Nano),
			LastTimestamp:   lastTime.Format(time.RFC3339Nano),
			HasLoginSuccess: hasLoginSuccess,
			Events:          evts,
		})
	}

	sort.Slice(ipInfos, func(i, j int) bool {
		return ipInfos[i].IP < ipInfos[j].IP
	})

	app := tview.NewApplication()
	pages := tview.NewPages()

	mainTable := tview.NewTable().SetBorders(true).SetFixed(1, 0)
	mainTable.SetCell(0, 0, tview.NewTableCell("SRC_IP").SetSelectable(false))
	mainTable.SetCell(0, 1, tview.NewTableCell("FIRST_EVENT").SetSelectable(false))
	mainTable.SetCell(0, 2, tview.NewTableCell("LAST_EVENT").SetSelectable(false))
	mainTable.SetCell(0, 3, tview.NewTableCell("LOGIN_SUCCESS?").SetSelectable(false))

	for i, info := range ipInfos {
		r := i + 1
		mainTable.SetCell(r, 0, tview.NewTableCell(info.IP))
		mainTable.SetCell(r, 1, tview.NewTableCell(info.FirstTimestamp))
		mainTable.SetCell(r, 2, tview.NewTableCell(info.LastTimestamp))
		mainTable.SetCell(r, 3, tview.NewTableCell(fmt.Sprint(info.HasLoginSuccess)))
	}

	detailTable := tview.NewTable().SetBorders(true).SetFixed(1, 0)
	detailTable.SetCell(0, 0, tview.NewTableCell("TIMESTAMP").SetSelectable(false))
	detailTable.SetCell(0, 1, tview.NewTableCell("EVENTID").SetSelectable(false))
	detailTable.SetCell(0, 2, tview.NewTableCell("USERNAME/PWD").SetSelectable(false))
	detailTable.SetCell(0, 3, tview.NewTableCell("INPUT").SetSelectable(false))
	detailTable.SetCell(0, 4, tview.NewTableCell("MESSAGE").SetSelectable(false))

	showDetail := func(info IPInfo) {
		for i := detailTable.GetRowCount() - 1; i >= 1; i-- {
			detailTable.RemoveRow(i)
		}
		row := 1
		for _, evt := range info.Events {
			timestamp, _ := evt["timestamp"].(string)
			eventid, _ := evt["eventid"].(string)
			message, _ := evt["message"].(string)
			username, _ := evt["username"].(string)
			password, _ := evt["password"].(string)
			inputCmd, _ := evt["input"].(string)

			userinfo := ""
			if username != "" || password != "" {
				userinfo = fmt.Sprintf("%s/%s", username, password)
			}

			detailTable.SetCell(row, 0, tview.NewTableCell(timestamp))
			detailTable.SetCell(row, 1, tview.NewTableCell(eventid))
			detailTable.SetCell(row, 2, tview.NewTableCell(userinfo))
			detailTable.SetCell(row, 3, tview.NewTableCell(inputCmd))
			detailTable.SetCell(row, 4, tview.NewTableCell(message))
			row++
		}
	}

	mainTable.SetSelectable(true, false).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			app.Stop()
		}
	}).SetSelectedFunc(func(row, col int) {
		if row > 0 && row-1 < len(ipInfos) {
			chosenIP := ipInfos[row-1]
			showDetail(chosenIP)
			pages.SwitchToPage("detail")
		}
	})
	detailTable.SetSelectable(true, false).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			pages.SwitchToPage("main")
		}
	}).SetSelectedFunc(func(row, col int) {
		if row > 0 {
			message := detailTable.GetCell(row, 4).Text
			if strings.HasPrefix(message, "Closing TTY Log: ") {
				command := extractTTYLogCommand(message)

				modal := tview.NewModal().SetText(fmt.Sprintf("Run with bin/playlog - Docker:\n%s", command)).
					AddButtons([]string{"Copy to Clipboard", "Close"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						if buttonLabel == "Copy to Clipboard" {
							clipboard.WriteAll(command)
						}
						pages.RemovePage("modal")
					})

				pages.AddPage("modal", modal, true, true)
			}
		}
	})

	pages.AddPage("main", mainTable, true, true)
	pages.AddPage("detail", detailTable, true, false)

	if err := app.SetRoot(pages, true).SetFocus(pages).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

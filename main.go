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

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CowrieEvent est un simple alias pour un événement Cowrie, stocké dans une map
type CowrieEvent map[string]interface{}

type IPInfo struct {
	IP              string
	FirstTimestamp  string
	LastTimestamp   string
	HasLoginSuccess bool
	Events          []CowrieEvent
}

func parseTimestamp(ts string) time.Time {
	// Les timestamps Cowrie sont souvent de la forme "2024-12-17T14:38:20.918891Z"
	// On tente un parse RFC3339
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		// En cas d'erreur, on renvoie un temps zéro
		return time.Time{}
	}
	return t
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <fichier.json>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur d'ouverture du fichier: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	eventsByIP := make(map[string][]CowrieEvent)

	// Lecture des events
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var evt CowrieEvent
		err := json.Unmarshal([]byte(line), &evt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erreur de parsing JSON : %v\n", err)
			continue
		}

		src_ip, _ := evt["src_ip"].(string)
		if src_ip == "" {
			src_ip = "UNKNOWN_IP"
		}
		eventsByIP[src_ip] = append(eventsByIP[src_ip], evt)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "Erreur de lecture du fichier : %v\n", err)
		os.Exit(1)
	}

	// Construction des infos par IP
	var ipInfos []IPInfo
	for ip, evts := range eventsByIP {
		var firstTime time.Time
		var lastTime time.Time
		hasLoginSuccess := false

		// On trie par timestamp pour être sûr de l'ordre
		sort.Slice(evts, func(i, j int) bool {
			t1 := parseTimestamp(fmt.Sprint(evts[i]["timestamp"]))
			t2 := parseTimestamp(fmt.Sprint(evts[j]["timestamp"]))
			return t1.Before(t2)
		})

		if len(evts) > 0 {
			firstTime = parseTimestamp(fmt.Sprint(evts[0]["timestamp"]))
			lastTime = parseTimestamp(fmt.Sprint(evts[len(evts)-1]["timestamp"]))
		}

		// Vérifier s'il y a eu un login.success
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

	// Tri par IP, juste pour un ordre déterministe
	sort.Slice(ipInfos, func(i, j int) bool {
		return ipInfos[i].IP < ipInfos[j].IP
	})

	app := tview.NewApplication()
	pages := tview.NewPages()

	// Table principale des IPs
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

	// Détail: On crée un tableau, mais on le remplit plus tard quand on aura choisi l'IP
	detailTable := tview.NewTable().SetBorders(true).SetFixed(1, 0)
	detailTable.SetCell(0, 0, tview.NewTableCell("TIMESTAMP").SetSelectable(false))
	detailTable.SetCell(0, 1, tview.NewTableCell("EVENTID").SetSelectable(false))
	detailTable.SetCell(0, 2, tview.NewTableCell("USERNAME/PWD").SetSelectable(false))
	detailTable.SetCell(0, 3, tview.NewTableCell("INPUT").SetSelectable(false))
	detailTable.SetCell(0, 4, tview.NewTableCell("MESSAGE").SetSelectable(false))

	// Fonction pour afficher le détail d'une IP dans detailTable
	showDetail := func(info IPInfo) {
		// On clear les lignes en dessous de l'en-tête
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

	// Navigation
	mainTable.SetSelectable(true, false).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			app.Stop()
		}
	}).SetSelectedFunc(func(row, col int) {
		if row > 0 && row-1 < len(ipInfos) {
			chosenIP := ipInfos[row-1]
			// on affiche le detail
			showDetail(chosenIP)
			pages.SwitchToPage("detail")
		}
	})

	detailTable.SetSelectable(true, false).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			// Retour à la liste IP
			pages.SwitchToPage("main")
		}
	})

	pages.AddPage("main", mainTable, true, true)
	pages.AddPage("detail", detailTable, true, false)

	if err := app.SetRoot(pages, true).SetFocus(pages).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur interface TUI: %v\n", err)
		os.Exit(1)
	}
}

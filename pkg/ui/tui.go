package ui

import (
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/gdamore/tcell/v2"
	"github.com/openshift/pagerduty-short-circuiter/pkg/client"
	pdcli "github.com/openshift/pagerduty-short-circuiter/pkg/pdcli/alerts"
	"github.com/openshift/pagerduty-short-circuiter/pkg/utils"
	"github.com/rivo/tview"
)

type TUI struct {

	// Main UI elements
	App                 *tview.Application
	AlertMetadata       *tview.TextView
	Table               *tview.Table
	IncidentsTable      *tview.Table
	NextOncallTable     *tview.Table
	AllTeamsOncallTable *tview.Table
	Pages               *tview.Pages
	LogWindow           *tview.TextView
	Layout              *tview.Flex
	Footer              *tview.TextView
	ServiceLogView      *tview.TextView
	FrontPage           string

	// API related
	Client       client.PagerDutyClient
	IncidentOpts pagerduty.ListIncidentsOptions
	Alerts       []pdcli.Alert

	// Internals
	SelectedIncidents map[string]string
	Incidents         [][]string
	AckIncidents      []string
	AssignedTo        string
	Username          string
	Role              string
	Columns           string
	ClusterID         string
	ClusterName       string
	CurrentOnCallPage int

	// SOP Related
	SOPLink  string
	NumLinks int
	SOPView  *tview.TextView

	// Multi-Window Terminals Related
	TerminalLayout      *tview.Flex
	TerminalPages       *tview.Pages
	TerminalPageBar     *tview.TextView
	TerminalFixedFooter *tview.TextView
	TerminalTabs        []TerminalTab
	TerminalUIRegionIDs []int
	TerminalInputBuffer []rune
	TerminalLastChars   []rune
}

// InitAlertsUI initializes TUI table component.
// It adds the returned table as a new TUI page view.
func (tui *TUI) InitAlertsUI(alerts []pdcli.Alert, tableTitle string, pageTitle string) {
	headers, data := pdcli.GetTableData(alerts, tui.Columns)
	tui.Table = tui.InitTable(headers, data, true, false, tableTitle)
	tui.SetAlertsTableEvents(alerts)

	if len(alerts) == 0 && tui.Username == tui.AssignedTo {
		utils.InfoLogger.Printf("No acknowledged alerts for user %s found", tui.Username)
	}

	tui.Pages.AddPage(pageTitle, tui.Table, true, true)
	tui.FrontPage = pageTitle

	if pageTitle == TrigerredAlertsPageTitle {
		tui.Footer.SetText(FooterTextTrigerredAlerts)
	} else {
		tui.Footer.SetText(FooterTextAlerts)
	}
}

// InitIncidentsUI initializes TUI table component.
// It adds the returned table as a new TUI page view.
func (tui *TUI) InitIncidentsUI(incidents [][]string, tableTitle string, pageTitle string, isAckTable bool) {
	incidentHeaders := []string{"INCIDENT ID", "NAME", "SEVERITY", "STATUS", "SERVICE", "ASSIGNED TO"}

	if !isAckTable {
		tui.IncidentsTable = tui.InitTable(incidentHeaders, incidents, true, true, tableTitle)
		tui.SetIncidentsTableEvents()
	} else {
		tui.IncidentsTable = tui.InitTable(incidentHeaders, incidents, true, true, tableTitle)
		tui.SetAckTableEvents()
	}

	if !tui.Pages.HasPage(pageTitle) {
		tui.Pages.AddPage(pageTitle, tui.IncidentsTable, true, false)
	}
}

func (tui *TUI) InitEmptyIncidentsView(tableTitle string, pageTitle string) {
	// Create a TextView
	text := tview.NewTextView().
		SetText("No Incidents! No news is good news").
		SetTextAlign(tview.AlignCenter)
	text.SetDrawFunc(func(screen tcell.Screen, x, y, w, h int) (int, int, int, int) {
		y += h / 2
		return x, y, w, h
	}).SetBorder(true).SetTitle(tableTitle)

	if !tui.Pages.HasPage(pageTitle) {
		tui.Pages.AddPage(pageTitle, text, true, false)
	}
}

// TODO: Move this to new Footer + Help Combined
// func (tui *TUI) InitOnCallSecondaryView(user string, primary string, secondary string) {
// 	tui.SecondaryWindow.SetText(
// 		fmt.Sprintf("Logged in user: %s\nCurrent Primary on-call: %s\nCurrent Secondary on-call: %s",
// 			user,
// 			primary,
// 			secondary),
// 	)
// }

// initFooter initializes the footer text depending on the page currently visible.
func (t *TUI) initFooter() {
	name, _ := t.Pages.GetFrontPage()

	switch name {
	case AckIncidentsPageTitle:
		t.Footer.SetText(FooterTextAckIncidents).SetTextColor(PromptTextColor)

	default:
		t.Footer.SetText(FooterText).SetTextColor(PromptTextColor)
	}

	if strings.Contains(name, OncallPageTitle) {
		t.Footer.SetText(FooterTextOncall).SetTextColor(PromptTextColor)
	}
}

// Init initializes all the TUI main elements.
func (tui *TUI) Init() {
	tui.App = tview.NewApplication()
	tui.Pages = tview.NewPages()
	tui.LogWindow = tview.NewTextView()
	tui.Footer = tview.NewTextView()
	tui.AlertMetadata = tview.NewTextView()
	tui.ServiceLogView = tview.NewTextView()
	tui.TerminalPages = tview.NewPages()
	tui.TerminalPageBar = tview.NewTextView()
	tui.TerminalFixedFooter = tview.NewTextView()

	tui.SOPView = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetChangedFunc(func() {
			tui.App.Draw()
		})

	tui.LogWindow.
		SetChangedFunc(func() { tui.App.Draw() }).
		SetScrollable(true).
		ScrollToEnd().
		SetBorder(true).
		SetBorderColor(BorderColor).
		SetBorderAttributes(tcell.AttrDim).
		SetBorderPadding(0, 0, 1, 1).
		SetTitle(fmt.Sprintf(TitleFmt, KiteLogsTableTitle))

	tui.Footer.
		SetTextAlign(tview.AlignLeft).
		SetTextColor(FooterTextColor).
		SetBorderPadding(1, 0, 1, 1)

	tui.TerminalFixedFooter.
		Clear().SetBackgroundColor(TerminalFooterTextColor)

	tui.AlertMetadata.
		SetScrollable(true).
		SetBorder(true).
		SetBorderColor(BorderColor).
		SetBorderPadding(1, 1, 1, 1).
		SetBorderAttributes(tcell.AttrDim).
		SetTitle(fmt.Sprintf(TitleFmt, AlertMetadataViewTitle))

	tui.ServiceLogView.
		SetScrollable(true).
		SetBorder(true).
		SetBorderColor(BorderColor).
		SetBorderPadding(1, 1, 1, 1).
		SetBorderAttributes(tcell.AttrDim).
		SetTitle(fmt.Sprintf(TitleFmt, ServiceLogsPageTitle))

	// Initialize logger to output to log view
	utils.InitLogger(tui.LogWindow)

	// Create the main layout
	tui.Layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tui.Pages, 0, 9, true).
		AddItem(tui.Footer, 0, 1, false)

	kiteTab := InitKiteTab(tui, tui.Layout)
	tui.TerminalLayout = InitTerminalMux(tui, kiteTab)
}

// StartApp sets the UI layout and renders all the TUI elements.
func (t *TUI) StartApp() error {
	t.initFooter()
	t.initKeyboard()

	return t.App.SetRoot(t.TerminalLayout, true).EnableMouse(false).Run()
}

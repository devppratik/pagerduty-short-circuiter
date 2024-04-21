package ui

import (
	"fmt"
	"strings"

	"os"
	"os/exec"
	"strconv"

	pdApi "github.com/PagerDuty/go-pagerduty"
	"github.com/gdamore/tcell/v2"

	"github.com/openshift/pagerduty-short-circuiter/pkg/client"
	"github.com/openshift/pagerduty-short-circuiter/pkg/constants"
	pdcli "github.com/openshift/pagerduty-short-circuiter/pkg/pdcli/alerts"
	"github.com/openshift/pagerduty-short-circuiter/pkg/utils"
)

// initKeyboard initializes the keyboard event handlers for all the TUI components.
func (tui *TUI) initKeyboard() {
	var isEscapeSequence bool = false
	tui.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isEscapeSequence {
			if event.Rune() >= '0' && event.Rune() <= '9' {
				slideNum, _ := strconv.Atoi(string(event.Rune()))
				SwitchToSlide(slideNum, tui)
			}
			tui.TerminalFixedFooter.
				SetText(TerminalFooterText).
				SetBackgroundColor(TerminalFooterTextColor)
			isEscapeSequence = false
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			// Check if alerts command is executed
			if tui.Pages.HasPage(AckIncidentsPageTitle) {
				page, _ := tui.Pages.GetFrontPage()

				// Handle page traversal
				switch page {
				case AlertDataPageTitle:
					tui.Pages.SwitchToPage(tui.FrontPage)
					tui.Footer.SetText(FooterTextAlerts)
				case ServiceLogsPageTitle:
					tui.Pages.SwitchToPage(AlertDataPageTitle)
				case AlertMetadata:
					tui.Pages.SwitchToPage(IncidentsPageTitle)
					tui.Footer.SetText(FooterTextAlerts)
				case AckAlertDataPage:
					tui.Pages.SwitchToPage(AckIncidentsPageTitle)
				default:
					// tui.InitAlertsUI(tui.Alerts, AlertsTableTitle, AlertsPageTitle)
					tui.Pages.SwitchToPage(AckIncidentsPageTitle)
					tui.Footer.SetText(FooterTextAckIncidents)
				}
			}
			// Check if oncall command is executed
			if title, _ := tui.Pages.GetFrontPage(); strings.Contains(title, "Oncall") {
				tui.Pages.SwitchToPage(fmt.Sprintf("%s%d", OncallPageTitle, 2))
				tui.Footer.SetText(FooterTextOncall)
			}
			return nil
		}
		if event.Key() == tcell.KeyLeft && CursorPos > 0 {
			CursorPos--
			return event
		} else if event.Key() == tcell.KeyRight && CursorPos < len(tui.TerminalInputBuffer) {
			CursorPos++
			return event
		} else if event.Key() == tcell.KeyCtrlN {
			NextSlide(tui)
			return nil
			// Move to the Previous Slide
		} else if event.Key() == tcell.KeyCtrlP {
			PreviousSlide(tui)
			return nil
			// Add a new Slide - bash
		} else if event.Key() == tcell.KeyCtrlS {
			AddNewSlide(tui, constants.Shell, os.Getenv("SHELL"), []string{}, false)
			return nil
			// Add a new Slide - ocm-container
		} else if event.Key() == tcell.KeyCtrlO {
			OcmContainerPath, err := exec.LookPath(constants.OcmContainer)
			if err != nil {
				utils.ErrorLogger.Println("ocm-container is not found.\nPlease install it via:", constants.OcmContainerURL)
				return nil
			}
			AddNewSlide(tui, constants.OcmContainer, OcmContainerPath, []string{}, false)
			return nil

		} else if event.Key() == tcell.KeyCtrlB {
			// Reset the input buffer
			tui.TerminalFixedFooter.
				SetText(TerminalFooterEscapeState).
				SetBackgroundColor(TerminalFooterEscapeStateColor)
			isEscapeSequence = true
			return nil
			// Delete the current active Slide
		} else if event.Key() == tcell.KeyCtrlE {
			slideNum, _ := strconv.Atoi(tui.TerminalPageBar.GetHighlights()[0])
			RemoveSlide(slideNum, tui)
			tui.TerminalInputBuffer = []rune{}
			return nil
		}
		// Override the default exit behaviour with Ctrl+C
		if event.Key() == tcell.KeyCtrlC {
			return nil
		}
		// Exit the App on Ctrl + Q
		if event.Key() == tcell.KeyCtrlQ {
			utils.InfoLogger.Println("Exiting kite")
			tui.App.Stop()
		}

		tui.setupAlertsPageInput()
		tui.setupIncidentsPageInput()
		tui.setupAlertDetailsPageInput()
		tui.setupOncallPageInput()

		return event
	})
}

func (tui *TUI) setupAlertsPageInput() {
	if title, _ := tui.Pages.GetFrontPage(); title == AckIncidentsPageTitle {

		tui.Pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

			if event.Rune() == '2' {
				utils.InfoLogger.Print("Switching to incidents view")
				tui.SeedIncidentsUI()

				if len(tui.Incidents) == 0 {
					utils.InfoLogger.Printf("No trigerred incidents assigned to found")
				}

				tui.Pages.SwitchToPage(IncidentsPageTitle)
			}

			// Alerts refresh
			if event.Rune() == 'r' || event.Rune() == 'R' {
				utils.InfoLogger.Print("Refreshing alerts...")
				tui.SeedAlertsUI()
			}

			// Kite Logs
			if event.Rune() == 'l' || event.Rune() == 'L' {
				utils.InfoLogger.Print("Switching to Logs...")
				tui.fetchKiteLogs()
			}

			return event
		})
	}
}

func (tui *TUI) setupIncidentsPageInput() {
	if title, _ := tui.Pages.GetFrontPage(); title == IncidentsPageTitle {
		tui.Pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlA {
				for _, v := range tui.SelectedIncidents {
					if v != "" {
						tui.AckIncidents = append(tui.AckIncidents, v)
					}
				}

				if len(tui.AckIncidents) == 0 {
					utils.ErrorLogger.Print("Please select atleast one incident to acknowledge")
				} else {
					tui.ackowledgeSelectedIncidents()
				}
			}
			if event.Rune() == 'V' || event.Rune() == 'v' {
				row, _ := tui.IncidentsTable.GetSelection()
				var incident pdApi.Incident
				client, _ := client.NewClient().Connect()
				incidentID := tui.IncidentsTable.GetCell(row, 0).Text
				incident.APIObject.ID = incidentID
				var clusterName string
				var alertData string

				alerts, _ := pdcli.GetIncidentAlerts(client, incident)
				Alert := alerts[0]

				for _, alert := range alerts {
					if incidentID == alert.IncidentID {
						alertData = pdcli.ParseAlertMetaData(alert)
						clusterName = alert.ClusterName
						tui.ClusterID = alert.ClusterID
						break
					}
				}
				if len(alerts) == 1 {
					alertData = pdcli.ParseAlertMetaData(Alert)
					tui.AlertMetadata.SetText(alertData)
					tui.Pages.AddAndSwitchToPage(AlertMetadata, tui.AlertMetadata, true)

				} else {
					tui.SetAlertsTableEvents(alerts)
					tui.InitAlertsUI(alerts, AlertMetadata, AlertMetadata)

				}
				// Do not prompt for cluster login if there's no cluster ID associated with the alert (v3 clusters)
				if tui.ClusterID != "N/A" && tui.ClusterID != "" && alertData != "" {
					FooterText := fmt.Sprintf("Press 'Y' to log into the cluster: %s | Press 'S' to view the SOP | Press 'L' to view service logs | [Esc] Go Back", clusterName)
					tui.Footer.SetText(FooterText)
				}
			}
			// Move to Ack Incidents
			if event.Rune() == '1' {
				utils.InfoLogger.Print("Switching to acknowledged incidents view")
				tui.SeedAckIncidentsUI()
				if len(tui.Incidents) == 0 {
					utils.InfoLogger.Printf("No acknowledged incidents assigned found")
				}
			}
			// Kite Logs
			if event.Rune() == 'l' || event.Rune() == 'L' {
				utils.InfoLogger.Print("Switching to Logs...")
				tui.fetchKiteLogs()
			}
			return event
		})
	}
}

func (tui *TUI) setupAlertDetailsPageInput() {
	tui.AlertMetadata.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		if event.Rune() == 'Y' || event.Rune() == 'y' {
			// Get ocm-conatiner executable from PATH
			ocmContainer, err := exec.LookPath("ocm-container")

			if err != nil {
				errMessage := "ocm-container is not found.\nPlease install it via: " + constants.OcmContainerURL
				utils.ErrorLogger.Print(errMessage)
				return nil
			}

			// Convert the ClusterID into args for ocm-container command
			clusterIDArgs := []string{tui.ClusterID}
			AddNewSlide(tui, tui.ClusterName, ocmContainer, clusterIDArgs, true)
		}

		if event.Rune() == 'L' || event.Rune() == 'l' {
			utils.InfoLogger.Print("Retrieving service logs for cluster")
			tui.fetchClusterServiceLogs()
		}

		if event.Rune() == 'S' || event.Rune() == 's' {
			if tui.SOPLink == "" || tui.SOPLink == "<nil>" {
				utils.InfoLogger.Print("No SOP mentioned for the alert")
				return nil
			}
			utils.InfoLogger.Print("Opening SOP in a new tab")
			ViewAlertSOP(tui, tui.SOPLink)
		}

		return event
	})
}

func (tui *TUI) setupOncallPageInput() {
	if title, _ := tui.Pages.GetFrontPage(); strings.Contains(title, OncallPageTitle) {
		tui.Pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

			if tui.NextOncallTable != nil {
				if event.Rune() == 'N' || event.Rune() == 'n' {
					utils.InfoLogger.Print("Viewing user next on-call schedule")
					tui.Pages.SwitchToPage(NextOncallPageTitle)

					if len(tui.AckIncidents) == 0 {
						utils.InfoLogger.Print("You are not scheduled for any oncall duties for the next 3 months. Cheer up!")
					}
				}
			}

			if tui.AllTeamsOncallTable != nil {
				if event.Rune() == 'A' || event.Rune() == 'a' {
					utils.InfoLogger.Print("Switching to all team on-call view")
					tui.Pages.SwitchToPage(AllTeamsOncallPageTitle)
				}
			}
			if event.Key() == tcell.KeyLeft {
				if tui.CurrentOnCallPage > 0 {
					tui.CurrentOnCallPage -= 1
				}
				tui.Pages.SwitchToPage(fmt.Sprintf("%s%d", OncallPageTitle, tui.CurrentOnCallPage))
			}
			if event.Key() == tcell.KeyRight {
				if tui.CurrentOnCallPage < 4 {
					tui.CurrentOnCallPage += 1
				}
				tui.Pages.SwitchToPage(fmt.Sprintf("%s%d", OncallPageTitle, tui.CurrentOnCallPage))
			}
			return event
		})
	}
}

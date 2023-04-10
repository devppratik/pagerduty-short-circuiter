package ui

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/gdamore/tcell/v2"

	"github.com/openshift/pagerduty-short-circuiter/pkg/constants"
	pdcli "github.com/openshift/pagerduty-short-circuiter/pkg/pdcli/alerts"
	"github.com/openshift/pagerduty-short-circuiter/pkg/utils"
)

// initKeyboard initializes the keyboard event handlers for all the TUI components.
func (tui *TUI) initKeyboard() {
	tui.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		if event.Key() == tcell.KeyEscape {
			// Check if alerts command is executed
			if tui.Pages.HasPage(AlertsPageTitle) {
				tui.InitAlertsSecondaryView()
				page, _ := tui.Pages.GetFrontPage()

				// If the user is viewing the alert metadata
				if page == AlertDataPageTitle {
					tui.Pages.SwitchToPage(tui.FrontPage)
				} else if page == HighAlertsPageTitle || page == LowAlertsPageTitle {
					tui.Pages.SwitchToPage(TrigerredAlertsPageTitle)
					tui.Footer.SetText(FooterTextTrigerredAlerts)
				} else {
					tui.InitAlertsUI(tui.Alerts, AlertsTableTitle, AlertsPageTitle)
					tui.Pages.SwitchToPage(AlertsPageTitle)
					tui.Footer.SetText(FooterTextAlerts)
				}
			}

			// Check if oncall command is executed
			if tui.Pages.HasPage(OncallPageTitle) {
				tui.Pages.SwitchToPage(OncallPageTitle)
				tui.Footer.SetText(FooterTextOncall)
			}

			return nil
		}

		if event.Key() == tcell.KeyCtrlN {
			NextSlide(tui)
			return nil
			// Move to the Previous Slide
		} else if event.Key() == tcell.KeyCtrlP {
			PreviousSlide(tui)
			return nil
			// Add a new Slide
		} else if event.Key() == tcell.KeyCtrlA {
			AddNewSlide(tui, "SHELL", os.Getenv("SHELL"), []string{}, false)
			return nil
			// Delete the current active Slide
		} else if event.Key() == tcell.KeyCtrlE {
			slideNum, _ := strconv.Atoi(tui.TerminalPageBar.GetHighlights()[0])
			RemoveSlide(slideNum, tui)
			return nil
			// TODO : Handle the buffer with more edge cases
			// Handling Backspace with input buffer
		} else if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if len(tui.TerminalInputBuffer) > 0 {
				tui.TerminalInputBuffer = tui.TerminalInputBuffer[:len(tui.TerminalInputBuffer)-1]
			}
			// Working on the input buffer
		} else if event.Key() == tcell.KeyRune {
			tui.TerminalInputBuffer = append(tui.TerminalInputBuffer, event.Rune())
			// Exit the current slide when exit command is typed
		} else if event.Key() == tcell.KeyEnter {
			if string(tui.TerminalInputBuffer) == "exit" {
				tui.TerminalInputBuffer = []rune{}
				slideNum, _ := strconv.Atoi(tui.TerminalPageBar.GetHighlights()[0])
				RemoveSlide(slideNum, tui)
			}
			tui.TerminalInputBuffer = []rune{}
		}

		if event.Rune() == 'q' || event.Rune() == 'Q' {
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
	if title, _ := tui.Pages.GetFrontPage(); title == AlertsPageTitle {

		tui.Pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

			if event.Rune() == '1' {
				utils.InfoLogger.Print("Switching to trigerred alerts view")
				tui.InitAlertsUI(pdcli.TrigerredAlerts, TrigerredAlertsTableTitle, TrigerredAlertsPageTitle)
			}

			if event.Rune() == '2' {
				utils.InfoLogger.Print("Switching to acknowledged incidents view")
				tui.SeedAckIncidentsUI()

				if len(tui.Incidents) == 0 {
					utils.InfoLogger.Printf("No acknowledged incidents assigned found")
				}

				tui.Pages.SwitchToPage(AckIncidentsPageTitle)
			}

			if event.Rune() == '3' {
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

		return event
	})
}

func (tui *TUI) setupOncallPageInput() {
	if title, _ := tui.Pages.GetFrontPage(); title == OncallPageTitle {
		tui.Table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

			if tui.NextOncallTable != nil {
				if event.Rune() == 'N' || event.Rune() == 'n' {
					utils.InfoLogger.Print("Viewing user next on-call schedule")
					tui.Pages.SwitchToPage(NextOncallPageTitle)
					tui.Footer.SetText(FooterText)

					if len(tui.AckIncidents) == 0 {
						utils.InfoLogger.Print("You are not scheduled for any oncall duties for the next 3 months. Cheer up!")
					}
				}
			}

			if tui.AllTeamsOncallTable != nil {
				if event.Rune() == 'A' || event.Rune() == 'a' {
					utils.InfoLogger.Print("Switching to all team on-call view")
					tui.Pages.SwitchToPage(AllTeamsOncallPageTitle)
					tui.Footer.SetText(FooterText)
				}
			}

			return event
		})
	}
}

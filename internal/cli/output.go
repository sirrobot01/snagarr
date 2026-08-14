package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirrobot01/snagarr/internal/client"
	"golang.org/x/term"
)

type printer struct {
	out   io.Writer
	color bool
}

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiRed   = "\x1b[31m"
	ansiGreen = "\x1b[32m"
	ansiAmber = "\x1b[33m"
	ansiBlue  = "\x1b[34m"
	ansiCyan  = "\x1b[36m"
)

func newPrinter(out io.Writer, noColor bool) printer {
	return printer{out: out, color: !noColor && colorEnabled(out)}
}

func colorEnabled(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func (p printer) paint(code, value string) string {
	if !p.color {
		return value
	}
	return code + value + ansiReset
}

func (p printer) success(value string) string { return p.paint(ansiGreen, value) }
func (p printer) heading(value string) string { return p.paint(ansiBold, value) }
func (p printer) muted(value string) string   { return p.paint(ansiDim, value) }

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func (p printer) captured(item client.Item) {
	fmt.Fprintf(p.out, "%s  %s\n", p.success("✓"), p.heading(item.Title))
	fmt.Fprintf(p.out, "   %s %s\n", p.muted(padRight("Status", 9)), p.status(item.Status))
	fmt.Fprintf(p.out, "   %s #%d\n", p.muted(padRight("Item", 9)), item.ID)
	if item.Note != nil && *item.Note != "" {
		fmt.Fprintf(p.out, "   %s %s\n", p.muted(padRight("Note", 9)), *item.Note)
	}
}

func (p printer) list(response client.ItemsResponse, width int) {
	if len(response.Items) == 0 {
		fmt.Fprintln(p.out, p.heading("YOUR LIST"))
		fmt.Fprintln(p.out)
		fmt.Fprintln(p.out, "No matching items.")
		fmt.Fprintln(p.out, p.muted("Try `snagarr snag \"Sinners\"`."))
		return
	}

	noun := "ITEMS"
	if response.Total == 1 {
		noun = "ITEM"
	}
	fmt.Fprintln(p.out, p.heading(fmt.Sprintf("YOUR LIST  ·  %d %s", response.Total, noun)))
	fmt.Fprintln(p.out)
	titleWidth := width - 38
	if titleWidth < 18 {
		titleWidth = 18
	}
	if titleWidth > 48 {
		titleWidth = 48
	}
	for _, item := range response.Items {
		title := padRight(truncate(item.Title, titleWidth), titleWidth)
		year := "—"
		if item.Year != nil {
			year = fmt.Sprint(*item.Year)
		}
		status := padRight(statusLabel(item.Status), 13)
		fmt.Fprintf(p.out, "%s  %s  %-4s  %s  %s\n",
			p.statusMark(item.Status), title, year, p.statusWithPadding(item.Status, status), relativeTime(item.CapturedAt))
	}
}

func (p printer) statusView(server string, status client.StatusResponse) {
	fmt.Fprintf(p.out, "%s  %s\n", p.success("●"), p.heading("SNAGARR IS ONLINE"))
	fmt.Fprintf(p.out, "   %s  ·  %s\n\n", server, status.Version)
	fmt.Fprintf(p.out, "   %-12s %d\n", "Ready", status.Counts.Ready)
	fmt.Fprintf(p.out, "   %-12s %d\n", "Pending", status.Counts.Pending)
	fmt.Fprintf(p.out, "   %-12s %d\n", "Needs review", status.Counts.NeedsReview)
	fmt.Fprintf(p.out, "   %-12s %d\n", "Archived", status.Counts.Archived)

	services := make([]string, 0, len(status.Services))
	for name, connected := range status.Services {
		if connected {
			services = append(services, displayService(name))
		}
	}
	sort.Strings(services)
	fmt.Fprintln(p.out)
	if len(services) == 0 {
		fmt.Fprintln(p.out, p.muted("   No services connected"))
	} else {
		fmt.Fprintf(p.out, "   %s  %s\n", p.muted("Services"), strings.Join(services, " · "))
	}
}

func (p printer) status(value string) string {
	label := statusLabel(value)
	switch value {
	case "available", "watched":
		return p.paint(ansiGreen, label)
	case "requested", "monitored":
		return p.paint(ansiBlue, label)
	case "needs_review":
		return p.paint(ansiAmber, label)
	default:
		return p.paint(ansiCyan, label)
	}
}

func (p printer) statusWithPadding(value, padded string) string {
	switch value {
	case "available", "watched":
		return p.paint(ansiGreen, padded)
	case "requested", "monitored":
		return p.paint(ansiBlue, padded)
	case "needs_review":
		return p.paint(ansiAmber, padded)
	default:
		return p.paint(ansiCyan, padded)
	}
}

func (p printer) statusMark(value string) string {
	switch value {
	case "available", "watched":
		return p.paint(ansiGreen, "●")
	case "needs_review":
		return p.paint(ansiAmber, "◆")
	case "requested", "monitored":
		return p.paint(ansiBlue, "●")
	default:
		return p.paint(ansiCyan, "○")
	}
}

func statusLabel(value string) string {
	switch value {
	case "available":
		return "Ready"
	case "watched":
		return "Watched"
	case "needs_review":
		return "Needs review"
	case "monitored":
		return "Monitored"
	case "requested":
		return "Requested"
	case "new":
		return "New"
	default:
		return value
	}
}

func displayService(value string) string {
	switch value {
	case "tmdb":
		return "TMDB"
	case "radarr":
		return "Radarr"
	case "sonarr":
		return "Sonarr"
	case "overseerr":
		return "Overseerr"
	case "ntfy":
		return "ntfy"
	case "library":
		return "Library"
	default:
		return value
	}
}

func outputWidth(out io.Writer) int {
	if file, ok := out.(*os.File); ok {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 80
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	delta := time.Since(value)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return "now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh", int(delta.Hours()))
	case delta < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(delta.Hours()/24))
	default:
		return value.Local().Format("2 Jan")
	}
}

func truncate(value string, width int) string {
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

func padRight(value string, width int) string {
	padding := width - utf8.RuneCountInString(value)
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/cncf/cora/internal/registry"
)

// buildServicesCmd returns the `cora services` command tree.
func buildServicesCmd(reg *registry.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "List and describe configured services",
	}
	cmd.AddCommand(buildServicesListCmd(reg))
	cmd.AddCommand(buildServicesDescribeCmd(reg))
	return cmd
}

// ── cora services list ────────────────────────────────────────────────────────

type serviceRow struct {
	Name    string `json:"service"`
	Title   string `json:"title"`
	Version string `json:"version"`
	BaseURL string `json:"base_url"`
	Cached  string `json:"cached"`
}

func buildServicesListCmd(reg *registry.Registry) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured services with their title and endpoint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries := reg.Entries()
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Name < entries[j].Name
			})

			rows := make([]serviceRow, 0, len(entries))
			for _, e := range entries {
				r := serviceRow{
					Name:    e.Name,
					Title:   "-",
					Version: "-",
					BaseURL: e.BaseURL,
					Cached:  "no",
				}

				doc, fetchedAt, err := e.LoadCached()
				if err == nil && doc != nil && doc.Info != nil {
					if doc.Info.Title != "" {
						r.Title = doc.Info.Title
					}
					if doc.Info.Version != "" {
						r.Version = doc.Info.Version
					}
					r.Cached = fmt.Sprintf("yes (%s ago)", humanAge(time.Since(fetchedAt)))
				}

				rows = append(rows, r)
			}

			if format == "json" {
				out, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}

			tw := tablewriter.NewWriter(os.Stdout)
			tw.SetHeader([]string{"SERVICE", "TITLE", "VERSION", "BASE URL", "CACHED"})
			tw.SetBorder(false)
			tw.SetColumnSeparator("  ")
			tw.SetHeaderLine(false)
			tw.SetAutoWrapText(false)
			tw.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			tw.SetAlignment(tablewriter.ALIGN_LEFT)
			for _, r := range rows {
				tw.Append([]string{r.Name, r.Title, r.Version, r.BaseURL, r.Cached})
			}
			tw.Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

// ── cora services describe <service> ─────────────────────────────────────────

func buildServicesDescribeCmd(reg *registry.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "describe <service>",
		Short: "Show detail for a service: title, version, description, and all endpoints",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := reg.Lookup(args[0])
			if err != nil {
				return err
			}

			// Use cache when available; fall back to full load (may hit network).
			doc, fetchedAt, err := entry.LoadCached()
			if err != nil || doc == nil {
				doc, err = entry.LoadSpec(cmd.Context())
				if err != nil {
					return fmt.Errorf("load spec for %q: %w", entry.Name, err)
				}
				fetchedAt = time.Now()
			}

			printServiceDetail(entry, doc, fetchedAt)
			return nil
		},
	}
}

func printServiceDetail(entry *registry.Entry, doc *openapi3.T, fetchedAt time.Time) {
	// ── Header ────────────────────────────────────────────────────────────────
	fmt.Printf("Service:  %s\n", entry.Name)
	if doc.Info != nil {
		if doc.Info.Title != "" {
			fmt.Printf("Title:    %s\n", doc.Info.Title)
		}
		if doc.Info.Version != "" {
			fmt.Printf("Version:  %s\n", doc.Info.Version)
		}
		if doc.Info.Description != "" {
			fmt.Printf("Desc:     %s\n", truncateStr(doc.Info.Description, 120))
		}
	}
	fmt.Printf("Base URL: %s\n", entry.BaseURL)
	fmt.Printf("Spec URL: %s\n", entry.SpecURL)
	fmt.Printf("Cached:   %s ago\n", humanAge(time.Since(fetchedAt)))

	// ── Resources & endpoints ─────────────────────────────────────────────────
	if doc.Paths == nil || doc.Paths.Len() == 0 {
		fmt.Println("\nNo endpoints defined in spec.")
		return
	}

	type endpoint struct {
		method  string
		path    string
		summary string
	}

	resourceMap := map[string][]endpoint{}
	var resourceOrder []string
	seenRes := map[string]bool{}

	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			var res string
			if len(op.Tags) > 0 {
				res = strings.ToLower(strings.ReplaceAll(op.Tags[0], " ", "-"))
			} else {
				res = lastPathSeg(path)
			}
			if !seenRes[res] {
				seenRes[res] = true
				resourceOrder = append(resourceOrder, res)
			}
			sum := op.Summary
			if sum == "" {
				sum = op.Description
			}
			resourceMap[res] = append(resourceMap[res], endpoint{
				method:  strings.ToUpper(method),
				path:    path,
				summary: sum,
			})
		}
	}
	sort.Strings(resourceOrder)

	fmt.Println("\nResources:")
	for _, res := range resourceOrder {
		eps := resourceMap[res]
		sort.Slice(eps, func(i, j int) bool {
			if eps[i].method != eps[j].method {
				return methodOrder(eps[i].method) < methodOrder(eps[j].method)
			}
			return eps[i].path < eps[j].path
		})

		fmt.Printf("  %s\n", res)
		for _, ep := range eps {
			sum := ""
			if ep.summary != "" {
				sum = "  " + truncateStr(ep.summary, 55)
			}
			fmt.Printf("    %-8s %-42s%s\n", ep.method, ep.path, sum)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// humanAge converts a duration to a human-readable age string.
func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// truncateStr shortens s to at most max runes, collapsing internal newlines.
func truncateStr(s string, max int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", " "), "\n", " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// lastPathSeg extracts the last non-param path segment (no .json extension).
func lastPathSeg(path string) string {
	clean := strings.TrimSuffix(path, ".json")
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if s := segs[i]; s != "" && !strings.HasPrefix(s, "{") {
			return strings.ToLower(s)
		}
	}
	return "root"
}

// methodOrder assigns a canonical sort order to HTTP methods.
func methodOrder(m string) int {
	switch m {
	case "GET":
		return 0
	case "POST":
		return 1
	case "PUT":
		return 2
	case "PATCH":
		return 3
	case "DELETE":
		return 4
	default:
		return 5
	}
}

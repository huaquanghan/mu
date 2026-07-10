package cli

import (
	"github.com/huaquanghan/mu/internal/audit"
	"github.com/spf13/cobra"
)

var (
	auditReport  bool
	auditJSON    bool
	auditDryRun  bool
	auditInclude []string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Diagnose cleanup issues and apply recommended fixes",
	Long: `Scan the system for reclaimable space and health pressure, then
guide you through applying safe fixes (clean targets and optimize steps).

Interactive (default on a TTY):
  scan → select findings → confirm → apply → re-score

Scripting:
  mu audit --report     human-readable report (exit 1=warning, 2=critical)
  mu audit --json       structured JSON report
  mu audit --dry-run    wizard/report apply path without making changes

Opt-in clean categories (browser-cache, docker) appear in the audit but are
not selected unless listed in --include.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAudit()
	},
}

func init() {
	auditCmd.Flags().BoolVar(&auditReport, "report", false, "Print findings and exit (no apply)")
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Print JSON findings and exit")
	auditCmd.Flags().BoolVar(&auditDryRun, "dry-run", false, "Preview apply actions without making changes")
	auditCmd.Flags().StringSliceVar(&auditInclude, "include", nil, "Pre-select opt-in clean categories (e.g. browser-cache)")
}

func runAudit() error {
	return audit.Run(audit.Options{
		Report:  auditReport,
		JSON:    auditJSON,
		DryRun:  auditDryRun,
		Debug:   debug,
		Include: auditInclude,
	})
}

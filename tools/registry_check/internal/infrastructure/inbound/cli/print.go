package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

// Print writes the report: the summary, the table of capabilities, the requirements that wait for a task grouped by
// area, the ones no task mentions, the maturity of the library and the result.
func Print(output_writer io.Writer, report domain.Report) {
	fmt.Fprintf(output_writer, "registry: %s\n", report.RegistryPath)
	fmt.Fprintf(output_writer, "specification: %s: %d requirement identifiers in %d areas\n", report.SpecPath, report.Total, len(report.Areas))
	fmt.Fprintf(output_writer, "  %s\n", areas_line(report.Areas))
	fmt.Fprintf(output_writer, "requirements: %d total, %d covered by tests, %d pending, %d no task yet, %d not accounted for\n",
		report.Total, report.Covered, len(report.Pending), len(report.Unassigned), report.Unaccounted)
	fmt.Fprintf(output_writer, "capabilities: %d\n", len(report.Capabilities))
	for _, capability := range report.Capabilities {
		fmt.Fprintf(output_writer, "  %-24s %-6s %-12s go=%-15s python=%-15s covered %d/%d\n", capability.ID, capability.Level,
			capability.Status, capability.Go, capability.Python, capability.Covered, capability.Requirements)
	}
	fmt.Fprintf(output_writer, "not covered by a test yet, waiting for a task (%d):\n", len(report.Pending))
	print_waiting(output_writer, report.Pending)
	fmt.Fprintln(output_writer, "no task yet:")
	print_waiting(output_writer, report.Unassigned)
	fmt.Fprintf(output_writer, "%s\n", maturity_line(report.Maturity))
	if report.Passed() {
		fmt.Fprintln(output_writer, "result: PASSED")
	} else {
		fmt.Fprintf(output_writer, "result: FAILED (%d errors, listed below)\n", len(report.Findings))
	}
}

func areas_line(areas []domain.AreaCount) string {
	parts := make([]string, 0, len(areas))
	for _, area := range areas {
		parts = append(parts, fmt.Sprintf("%s %d", area.Area, area.Count))
	}
	return strings.Join(parts, ", ")
}

// print_waiting prints the requirements one per line, a heading before each area.
func print_waiting(output_writer io.Writer, waiting []domain.Waiting) {
	current_area := ""
	for _, requirement := range waiting {
		if requirement.Area != current_area {
			current_area = requirement.Area
			fmt.Fprintf(output_writer, "  area %s:\n", current_area)
		}
		fmt.Fprintf(output_writer, "    %s (%s): %s\n", requirement.ID, requirement.Area, waiting_text(requirement))
	}
}

func waiting_text(requirement domain.Waiting) string {
	if requirement.Pending == domain.PendingUnassigned {
		return domain.PendingUnassigned
	}
	return "pending " + requirement.Pending
}

func maturity_line(maturity domain.Maturity) string {
	if maturity.Capability == "" {
		return fmt.Sprintf("library maturity: %s (no capability is listed)", maturity.Level)
	}
	return fmt.Sprintf("library maturity: %s (least complete capability: %s, %s)", maturity.Level, maturity.Capability, maturity.Status)
}

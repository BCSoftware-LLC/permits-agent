package platform

import (
	"fmt"
	"net/url"
	"strings"
)

func validateEvidenceURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(v) > 2000 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("source_url must be an HTTPS URL without credentials")
	}
	return nil
}
func plain(s string) string {
	// Quotes and note text are untrusted. Escape Markdown/HTML syntax in exports.
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#")
	return r.Replace(s)
}
func Packet(c Case) string {
	p := BuildPlan(c.Intake)
	var b strings.Builder
	fmt.Fprintf(&b, "# Permits Agent preparation brief\n\nDRAFT — owner review required. This is a preparation worksheet, not a completed government application.\n\nCase: %s · revision %d\n\n## Business\n\n%s\n\nBusiness type: %s\n\nPremises: %s, %s, %s County, California\n\nDeclared jurisdiction: %s (not independently verified)\n\n", c.ID, c.Version, plain(c.Intake.BusinessName), plain(c.Intake.BusinessType), plain(c.Intake.Address), plain(c.Intake.City), plain(c.Intake.County), plain(c.Intake.Jurisdiction))
	fmt.Fprintf(&b, "## Coverage\n\n%s\n\n## Questions to resolve\n\n", p.Coverage)
	for _, q := range p.Questions {
		fmt.Fprintf(&b, "- [ ] %s\n", q)
	}
	b.WriteString("\n## Research and preparation checklist\n\n")
	for _, s := range p.Steps {
		fmt.Fprintf(&b, "### %s\n\nAuthority: %s. Status: needs verification.\n\n%s\n\nSources: %s\n\n", s.Title, s.Authority, s.Action, strings.Join(s.SourceIDs, ", "))
	}
	b.WriteString("## Recorded information\n\nAll notes are user-provided statements, not verified agency findings. A recorded receipt does not independently prove acceptance or approval.\n\n")
	for _, n := range c.Notes {
		fmt.Fprintf(&b, "- %s (%s): %s\n", n.RecordedAt, n.Kind, plain(n.Text))
		if n.SourceURL != "" {
			fmt.Fprintf(&b, "  Source supplied by user: %s\n", plain(n.SourceURL))
		}
	}
	b.WriteString("\n## Correspondence draft\n\nRecipient: [Confirm responsible agency and contact]\n\nSubject: Request to confirm permit and application requirements\n\n")
	fmt.Fprintf(&b, "We are preparing %s at %s, %s, California. Proposed activity: %s. Please confirm your jurisdiction, applicable approvals, current forms and instructions, required attachments, fees, appointment process and any prerequisites. Please identify the official source and any site-specific conditions.\n\n[Owner or authorized representative name]\n\nThis draft has not been sent.\n\n## Official discovery sources\n\n", plain(c.Intake.BusinessName), plain(c.Intake.Address), plain(c.Intake.City), plain(c.Intake.BusinessType))
	for _, s := range p.Sources {
		fmt.Fprintf(&b, "- %s: %s\n", s.Title, s.URL)
	}
	b.WriteString("\n## Filing review gate\n\n- [ ] Responsible agency and parcel jurisdiction confirmed\n- [ ] Applicability, current form revision and instructions verified\n- [ ] Every required field and supporting attachment reviewed\n- [ ] Fees and actual deadlines confirmed with the agency\n- [ ] Applicant signing and representative authority documented\n- [ ] Owner approves the exact final package and submission\n- [ ] Actual agency receipt captured after submission\n\nNo signature, filing, payment, appointment or message is executed by this export.\n")
	return b.String()
}

// Package platform provides tenant-scoped preparation workflows shared by the
// website and remote agents. Plans are research checklists, not legal findings.
package platform

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

type Source struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Coverage string `json:"coverage"`
}

// These are discovery entry points. Listing a source is not verification of a
// particular business's obligations or of the contents of a linked form.
var Sources = []Source{
	{"calgold", "CalGold permit discovery", "https://www.calgold.ca.gov/", "Statewide discovery; the publisher warns results may be incomplete"},
	{"sos", "California business formation", "https://www.sos.ca.gov/business-programs/business-entities/starting-business-checklist", "Entity and startup guidance"},
	{"tax", "California business permits and taxes", "https://taxes.ca.gov/small-business-assistance-center/starting-your-business-permits/", "State startup guidance"},
	{"cdtfa", "CDTFA permits and licenses", "https://cdtfa.ca.gov/services/permits-licenses.htm", "Sales and specialized tax registration"},
	{"abc", "California ABC licensing", "https://www.abc.ca.gov/licensing/", "Alcohol license discovery; live mirror and reference tools available"},
	{"abc-forms", "ABC forms", "https://www.abc.ca.gov/licensing/license-forms/", "Official forms index; no automatic completion or submission"},
	{"ttb", "TTB Permits Online", "https://www.ttb.gov/online-services/online-permits", "Federal alcohol permit, notice and registration discovery"},
	{"ttb-retail", "TTB alcohol FAQs", "https://www.ttb.gov/faqs/alcohol?tid=7", "Distinguish dealer registration from permit categories"},
	{"la-planning", "Los Angeles Restaurant Beverage Program", "https://planning.lacity.gov/restaurant-beverage-program", "City of Los Angeles only; eligibility needs site review"},
	{"sd-planning", "San Diego alcohol zoning guidance", "https://www.sandiego.gov/development-services/forms-publications/information-bulletins/143", "City of San Diego only; eligibility needs site review"},
}

type Intake struct {
	BusinessName string   `json:"business_name"`
	BusinessType string   `json:"business_type"`
	Address      string   `json:"address"`
	City         string   `json:"city"`
	County       string   `json:"county"`
	Jurisdiction string   `json:"jurisdiction"` // city, unincorporated, unknown
	Activities   []string `json:"activities"`
}

var activities = map[string]bool{"alcohol_retail": true, "alcohol_production": true, "alcohol_wholesale": true, "alcohol_import": true, "food": true, "employees": true, "construction": true, "retail": true, "other": true}

func (i Intake) Validate() error {
	for _, v := range []string{i.BusinessName, i.BusinessType, i.Address, i.City, i.County} {
		if len(v) > 300 || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return fmt.Errorf("intake fields must be at most 300 characters with no control characters")
		}
	}
	if strings.TrimSpace(i.BusinessType) == "" {
		return fmt.Errorf("business_type is required")
	}
	if strings.TrimSpace(i.City) == "" && strings.TrimSpace(i.County) == "" {
		return fmt.Errorf("a California city or county is required")
	}
	if i.Jurisdiction != "city" && i.Jurisdiction != "unincorporated" && i.Jurisdiction != "unknown" {
		return fmt.Errorf("jurisdiction must be city, unincorporated, or unknown")
	}
	if len(i.Activities) > len(activities) {
		return fmt.Errorf("too many activities")
	}
	for _, a := range i.Activities {
		if !activities[a] {
			return fmt.Errorf("unsupported activity: %s", a)
		}
	}
	return nil
}

type Step struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Authority string   `json:"authority"`
	Action    string   `json:"action"`
	SourceIDs []string `json:"source_ids"`
	DependsOn []string `json:"depends_on"`
	Status    string   `json:"status"`
}
type Plan struct {
	Version   string   `json:"version"`
	Coverage  string   `json:"coverage"`
	Steps     []Step   `json:"steps"`
	Questions []string `json:"questions"`
	Sources   []Source `json:"sources"`
}

func BuildPlan(i Intake) Plan {
	p := Plan{Version: "2026-09-20.1", Coverage: "California research and preparation checklist. Applicability, agency jurisdiction, current forms, fees, deadlines and completeness are unverified until reviewed for this premises. No filings are submitted.", Steps: []Step{}, Questions: []string{}, Sources: []Source{}}
	used := map[string]bool{}
	add := func(id, title, authority, action string, sources, dependencies []string) {
		p.Steps = append(p.Steps, Step{id, title, authority, action, sources, dependencies, "needs_verification"})
		for _, s := range sources {
			used[s] = true
		}
	}
	add("jurisdiction", "Confirm the premises and governing jurisdiction", "City or county planning department", "Verify the parcel, incorporated boundary, zoning designation and proposed use with the responsible planning authority. A mailing city alone does not establish jurisdiction.", []string{"calgold", "sos"}, []string{})
	add("entity", "Choose and register the business structure", "California Secretary of State / county clerk / tax authorities", "Review entity registration, assumed-name filing, tax identifiers and ownership information for the chosen structure. Confirm which steps apply.", []string{"sos", "tax"}, []string{})
	add("local", "Identify local business approvals", "Responsible city or county", "Use CalGold as a starting point, then confirm the issuer's business license, zoning, CUP or administrative approval, building, fire, signage and accessibility review paths. Do not assume every business needs a CUP.", []string{"calgold"}, []string{"jurisdiction"})
	has := func(a string) bool {
		for _, v := range i.Activities {
			if a == v {
				return true
			}
		}
		return false
	}
	if has("retail") || has("food") || has("alcohol_retail") || has("alcohol_wholesale") || has("alcohol_production") || has("alcohol_import") {
		add("tax", "Check sales and activity-specific tax accounts", "CDTFA and applicable tax authorities", "Confirm taxable activities and required registrations directly with the agency. Record applicable accounts, current forms and renewals.", []string{"cdtfa", "tax"}, []string{"entity"})
	}
	if has("food") {
		add("health", "Confirm food facility and health approvals", "Local environmental health authority", "Identify the health authority for the exact premises. Ask about food facility permits, plan review, inspections and activity-specific requirements.", []string{"calgold"}, []string{"jurisdiction"})
	}
	alcohol := has("alcohol_retail") || has("alcohol_production") || has("alcohol_wholesale") || has("alcohol_import")
	if alcohol {
		add("abc", "Determine the California alcohol license path", "California ABC", "Confirm license type, transaction type, premises conditions and current forms. Use ABC reference tools and record any local prerequisites.", []string{"abc", "abc-forms"}, []string{"jurisdiction", "entity"})
		add("ttb", "Determine the federal alcohol path", "TTB", "Distinguish retail dealer registration from producer notices and wholesale/import permit paths. Confirm applicant signing authority and current instructions before preparing an application.", []string{"ttb", "ttb-retail"}, []string{"entity"})
		if strings.EqualFold(strings.TrimSpace(i.City), "Los Angeles") && i.Jurisdiction == "city" {
			add("la-alcohol", "Review Los Angeles alcohol land-use options", "Los Angeles City Planning", "Verify parcel jurisdiction and whether the Restaurant Beverage Program or another approval path applies; eligibility is not established by this checklist.", []string{"la-planning"}, []string{"jurisdiction"})
		}
		if strings.EqualFold(strings.TrimSpace(i.City), "San Diego") && i.Jurisdiction == "city" {
			add("sd-alcohol", "Review San Diego alcohol zoning verification", "San Diego Development Services", "Review the current zoning verification guidance for the exact premises and activity; confirm whether a CUP or another path applies.", []string{"sd-planning"}, []string{"jurisdiction"})
		}
	}
	if has("employees") {
		add("employment", "Review employer registrations and obligations", "California and federal employment authorities", "Identify employer accounts, insurance, workplace and industry-specific obligations using official agency guidance.", []string{"tax", "calgold"}, []string{"entity"})
	}
	if has("construction") {
		add("construction", "Review construction and occupancy approvals", "Local building and fire authorities", "Confirm plan review, construction permits, inspections and occupancy prerequisites for the proposed work before scheduling opening.", []string{"calgold"}, []string{"jurisdiction"})
	}
	add("specialty", "Check additional industry and premises requirements", "Relevant local, state and federal agencies", "Review professional licensing, environmental, fire, waste, entertainment, accessibility and other activity-specific approvals. This checklist does not exhaust industry requirements.", []string{"calgold", "sos"}, []string{"jurisdiction"})
	add("review", "Prepare the owner review package", "Business owner and authorized representative", "Collect current official forms and instructions, verify each required field, attach supporting evidence and obtain signatory review. Track submissions only after recording actual agency receipts.", []string{"calgold"}, []string{"local", "entity", "specialty"})
	if i.Address == "" {
		p.Questions = append(p.Questions, "What is the exact premises address and parcel identifier?")
	}
	if i.Jurisdiction == "unknown" {
		p.Questions = append(p.Questions, "Is the parcel inside an incorporated city or in unincorporated county territory?")
	}
	if len(i.Activities) == 0 {
		p.Questions = append(p.Questions, "Which activities will occur: sales, food, alcohol, construction, employment or other regulated services?")
	}
	p.Questions = append(p.Questions, "Is this a new business, ownership transfer, relocation or change to existing operations?", "Who owns the applicant entity and who has authority to sign?", "What is the intended opening date, and which agency deadlines have been confirmed?")
	for _, s := range Sources {
		if used[s.ID] {
			p.Sources = append(p.Sources, s)
		}
	}
	return p
}

func calendar(c Case, date string) (string, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("date must be YYYY-MM-DD")
	}
	// Date is an owner's planning reminder, never an inferred agency deadline.
	return fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Permits Agent//Review Reminder//EN\r\nBEGIN:VEVENT\r\nUID:%s-%s@permits-agent\r\nDTSTAMP:%s\r\nDTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\nSUMMARY:Review permit preparation case\r\nDESCRIPTION:Owner-selected planning reminder. Not an agency deadline.\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", c.ID, d.Format("20060102"), time.Now().UTC().Format("20060102T150405Z"), d.Format("20060102"), d.AddDate(0, 0, 1).Format("20060102")), nil
}

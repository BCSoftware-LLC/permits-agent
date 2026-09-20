package reference

import "sort"

const requirementsReviewed = "2026-09-06"

type reqEntry struct {
	Name                                 string
	Forms, Conditional, Documents, Notes []string
	Source                               string
	Reviewed                             string
}

var (
	newForms      = []string{"ABC-208-A", "ABC-208-B", "ABC-211-SIG", "ABC-217", "ABC-247", "ABC-251", "ABC-253", "ABC-255", "ABC-257", "ABC-258"}
	newCond       = []string{"ABC-140", "ABC-243", "ABC-256", "ABC-256-LLC"}
	newDocs       = []string{"State-issued identification, driver's license or passport for person(s) appearing at ABC to sign the application.", "ABC may request verification (proof) of the source of funds — e.g. bank statements, savings passbooks, loan papers, real estate papers, financial statement, gift/loan letters.", "Copy of Conditional Use Permit, or receipt showing the CUP application has been submitted (city/county planning department)."}
	newNotes      = []string{"The official page references Form ABC-211; ABC-211-SIG is a signature sheet rather than the complete application. Obtain the current application and filing instructions directly from ABC.", "Entity-specific forms also required depending on ownership structure: ABC-140 (corporations/LPs/LLCs), ABC-243 (corporations), ABC-256 (limited partnerships), ABC-256-LLC (limited liability companies).", "Confirm the available filing channel, posting period and notice obligations with the local ABC district office before submission.", "Check the current Responsible Beverage Service (RBS) requirements for the proposed on-premises alcohol service; this mapping does not determine applicability."}
	transferForms = []string{"ABC-208-A", "ABC-208-B", "ABC-211-SIG", "ABC-217", "ABC-227", "ABC-253", "ABC-257", "ABC-258", "ABC-211-A"}
	transferCond  = []string{"ABC-140", "ABC-243", "ABC-256", "ABC-256-LLC", "ABC-282"}
	transferDocs  = []string{"State-issued identification, driver's license or passport for person(s) appearing at ABC to sign the application.", "ABC may request verification (proof) of the source of funds — e.g. bank statements, savings passbooks, loan papers, real estate papers, financial statement, gift/loan letters."}
	transferNotes = []string{"Notice of Intended Transfer: Form ABC-227 (BPC §§24073/24074) must be recorded with the County Recorder and certified, then presented at filing (ABC-227-A is the §24071.1/24071.2 variant). Obtain the blank form from ABC or the escrow company.", "Escrow: if there is any purchase price or consideration, the full amount must be placed in escrow before filing (BPC §§24071.1, 24073, 24074, 24074.1, 24074.3, 24075); the applicant must furnish the §24074.3 statement under penalty of perjury within 30 days of application.", "Form ABC-282 (Temporary Permit) only if a temporary retail permit is requested — temporary permit fees apply.", "Entity-specific forms also required depending on ownership structure: ABC-140 (corporations/LPs/LLCs), ABC-243 (corporations), ABC-256 (limited partnerships), ABC-256-LLC (limited liability companies)."}
	renewDocs     = []string{"Renewal notice from ABC (mailed prior to the license expiration date, showing the annual fee, deadline and payment methods).", "Annual fee payment — online via ABC's Licensing Online Services portal (or by mail).", "Penalty fee if full payment is not received/postmarked by the payment deadline."}
	renewNotes    = []string{"All licenses renew on a 12-month basis; renewals must be paid on or before the last day of the month printed on the license. No license will be transferred until the renewal fee is paid.", "No renewal application form exists in the local forms index — renewal is an annual fee payment, not a form filing."}
	priority      = "'General' retail license types are quota-limited per county; new applications are filed only once per year through ABC's public 'Priority' lottery (form ABC-521, Priority License Application; resolve the current form with the forms command and see https://www.abc.ca.gov/licensing/priority-registration-drawings/)."
)

var perType = map[string][]string{
	"20": {}, "21": {priority}, "40": {}, "41": {}, "42": {}, "47": {priority}, "48": {priority}, "61": {},
	"58": {"Type 58 is a Caterer's Permit attached to an on-sale license, not a standalone license. Eligible holders: on-sale beer & wine (41, 42), on-sale general (47, 48, 57), club (50, 51, 52), on-sale general wine/food/art museum (78), general caterer's (83), designated special use (71, 72, 73, 87, 88, 99). Seasonal licensees and boat/vessel licensees are not eligible.", "Each catered event requires a separate Catering Authorization (Form ABC-218); a Supplemental Diagram (ABC-253) may be required for the event location, and events may need property owner / law enforcement approval."},
	"77": {"Type 77 is an Event Permit attached to an on-sale license, not a standalone license. Eligible holders: on-sale beer & wine (41, 42), on-sale general (47, 48, 49, 57, 90), on-sale general wine/food/art museum (78), designated special use (88, 99).", "Each event requires a separate Event Authorization (Form ABC-215); authorizations are limited to no more than four event days per calendar year."},
	"86": {"The Instructional Tasting License can only be held in conjunction with a qualified off-sale license; no dedicated application form was found in the official forms index — apply at the ABC district office and verify requirements with ABC."},
	"87": {"Type 87 (Special On-Sale General, specified census tracts in San Francisco) is limited in number and may only be issued to premises meeting BPC §23826.13; the licensee must operate and maintain the premises as a bona fide eating place."},
}

var requirements = buildRequirements()

func buildRequirements() map[[2]string]reqEntry {
	r := map[[2]string]reqEntry{}
	retail := map[string]string{"20": "Off-Sale Beer & Wine", "21": "Off-Sale General", "40": "On-Sale Beer", "41": "On-Sale Beer & Wine - Eating Place", "42": "On-Sale Beer & Wine - Public Premises", "47": "On-Sale General - Eating Place", "48": "On-Sale General - Public Premises", "61": "On-Sale Beer - Public Premises", "87": "Special On-Sale General License for Specified Census Tracts in the City/County of San Francisco"}
	for c, n := range retail {
		r[[2]string{c, "new"}] = reqEntry{n, newForms, newCond, newDocs, append(append([]string{}, newNotes...), perType[c]...), sourceURLs["new_license_application"], requirementsReviewed}
		r[[2]string{c, "transfer"}] = reqEntry{n, transferForms, transferCond, transferDocs, append(append([]string{}, transferNotes...), perType[c]...), sourceURLs["person_transfer"], requirementsReviewed}
		r[[2]string{c, "renewal"}] = reqEntry{n, nil, nil, renewDocs, append(append([]string{}, renewNotes...), perType[c]...), sourceURLs["fees"], requirementsReviewed}
	}
	r[[2]string{"58", "new"}] = reqEntry{"Caterer's Permit", []string{"ABC-239"}, nil, nil, append(perType["58"], "Apply for a Type 58 caterer's permit by submitting a completed Additional License/Permit Application (Form ABC-239) to the nearest ABC office. The permit carries its own annual fee."), sourceURLs["caterers_permit"], requirementsReviewed}
	r[[2]string{"58", "renewal"}] = reqEntry{"Caterer's Permit", nil, nil, renewDocs, append(append([]string{}, renewNotes...), append(perType["58"], "The caterer's permit carries its own annual fee, paid with the license renewal cycle.")...), sourceURLs["caterers_permit"], requirementsReviewed}
	r[[2]string{"77", "new"}] = reqEntry{"Event Permit", []string{"ABC-239"}, nil, nil, append(perType["77"], "Apply for a Type 77 event permit by submitting a completed Additional License/Permit Application (Form ABC-239) to the nearest ABC office. The permit carries its own annual fee."), sourceURLs["event_authorization"], requirementsReviewed}
	r[[2]string{"77", "renewal"}] = reqEntry{"Event Permit", nil, nil, renewDocs, append(append([]string{}, renewNotes...), append(perType["77"], "The event permit carries its own annual fee, paid with the license renewal cycle.")...), sourceURLs["event_authorization"], requirementsReviewed}
	r[[2]string{"86", "new"}] = reqEntry{"Instructional Tasting License", nil, nil, nil, perType["86"], sourceURLs["types"], requirementsReviewed}
	r[[2]string{"86", "renewal"}] = reqEntry{"Instructional Tasting License", nil, nil, renewDocs, append(append([]string{}, renewNotes...), perType["86"]...), sourceURLs["fees"], requirementsReviewed}
	return r
}

func mapped(code string) []string {
	out := []string{}
	for k := range requirements {
		if k[0] == code {
			out = append(out, k[1])
		}
	}
	sort.Strings(out)
	return out
}

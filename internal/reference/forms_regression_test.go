package reference

import (
	"context"
	"testing"
)

func TestFormsRetainNewAndAttributedRows(t *testing.T) {
	t.Setenv("ABC_AGENT_CACHE", t.TempDir())
	mockOfficial(t, `<table><tr class="entry"><th><a title="form" href="https://www.abc.ca.gov/wp-content/uploads/forms/ABC-140.pdf">ABC-140</a><span class="new">NEW</span></th><td>Mar-26</td><td>Tied-House Certification</td></tr><tr><th><a href="https://www.abc.ca.gov/wp-content/uploads/forms/ABC-521.pdf">ABC-521</a><span>NEW</span></th><td>Aug-26</td><td>Priority License Application</td></tr></table>`)
	got, err := Forms(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	forms := got.(FormsResult).Forms
	if len(forms) != 2 || forms[0].Number != "ABC-140" || forms[0].Title != "Tied-House Certification" {
		t.Fatalf("lost new forms: %+v", forms)
	}
}

package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
	"regexp"
)

// Validate arguments before opening local state or contacting a source.
func validateFlags(c *cobra.Command, args []string) error {
	str := func(k string) string { v, _ := c.Flags().GetString(k); return v }
	num := func(k string) int { v, _ := c.Flags().GetInt(k); return v }
	o := abc.QueryOptions{Limit: num("limit"), Offset: num("offset"), Status: str("status"), Type: str("type"), LicOrApp: str("lic-or-app"), ExpireYear: str("expires")}
	if err := abc.ValidateOptions(o); err != nil {
		return err
	}
	for _, k := range []string{"days", "min-days"} {
		if num(k) < 0 {
			return abc.ValidationError{Message: k + " must be nonnegative"}
		}
	}
	switch c.Name() {
	case "area", "pending", "overdue", "expiring", "digest":
		if err := abc.ValidateArea(abc.AreaFilter{ZIP: str("zip"), City: str("city"), County: str("county"), District: str("district"), Limit: num("limit"), Offset: num("offset"), MinDays: num("min-days")}); err != nil {
			return err
		}
	}
	file := str("watch")
	if c.Name() == "get" && len(args) == 1 {
		file = args[0]
	}
	if file != "" && !regexp.MustCompile(`^[0-9]{8}$`).MatchString(file) {
		return abc.ValidationError{Message: "file number must be exactly eight digits"}
	}
	return nil
}

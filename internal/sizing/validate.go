package sizing

import "fmt"

var knownApps = map[string]bool{"enterprise_security": true, "itsi": true}
var knownRegions = map[string]bool{"ap-southeast-2": true, "us-east-1": true, "us-west-2": true, "eu-west-1": true}

// Validate returns a list of problems. An empty list means the input is OK.
func (s Subscription) Validate(r Rules) []string {
	var errs []string
	if s.CustomerID == "" {
		errs = append(errs, "customer_id is required")
	}
	if s.Cloud != "aws" {
		errs = append(errs, "cloud must be \"aws\" (other clouds are not supported yet)")
	}
	if !knownRegions[s.Region] {
		errs = append(errs, fmt.Sprintf("region %q is not supported", s.Region))
	}
	if s.IngestGBPerDay <= 0 || s.IngestGBPerDay > r.MaxIngestGBPerDay {
		errs = append(errs, fmt.Sprintf("ingest_gb_per_day must be between 0 and %.0f", r.MaxIngestGBPerDay))
	}
	if s.RetentionDays < 1 || s.RetentionDays > r.MaxRetentionDays {
		errs = append(errs, fmt.Sprintf("retention_days must be between 1 and %d", r.MaxRetentionDays))
	}
	if _, ok := r.IndexerCapacity[s.SearchProfile]; !ok || s.SearchProfile == "enterprise_security" {
		errs = append(errs, "search_profile must be light, moderate or heavy")
	}
	if s.ConcurrentUsers < 0 {
		errs = append(errs, "concurrent_users cannot be negative")
	}
	for _, app := range s.PremiumApps {
		if !knownApps[app] {
			errs = append(errs, fmt.Sprintf("premium app %q is not supported", app))
		}
	}
	return errs
}

package sizing

import "testing"

func loadTestRules(t *testing.T) Rules {
	t.Helper()
	r, err := LoadRules("../../rules/v1.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return r
}

func TestSize(t *testing.T) {
	rules := loadTestRules(t)
	tests := []struct {
		name          string
		in            Subscription
		wantIndexers  int
		wantCoreSH    int
		wantPremiumSH int
		wantLocalTB   float64
		wantObjectTB  float64
	}{
		{
			name: "golden: 500 GB/day with ES, SmartStore, HA",
			in: Subscription{IngestGBPerDay: 500, RetentionDays: 90, SearchProfile: "moderate",
				ConcurrentUsers: 50, PremiumApps: []string{"enterprise_security"},
				SmartStore: true, HighAvailability: true},
			wantIndexers: 5, wantCoreSH: 3, wantPremiumSH: 1, wantLocalTB: 19.8, wantObjectTB: 22.5,
		},
		{
			name:         "small customer, no HA",
			in:           Subscription{IngestGBPerDay: 50, RetentionDays: 30, SearchProfile: "light", ConcurrentUsers: 5},
			wantIndexers: 1, wantCoreSH: 1, wantPremiumSH: 0, wantLocalTB: 2.0, wantObjectTB: 0,
		},
		{
			name:         "HA raises indexers to the minimum",
			in:           Subscription{IngestGBPerDay: 100, RetentionDays: 30, SearchProfile: "light", ConcurrentUsers: 10, HighAvailability: true},
			wantIndexers: 3, wantCoreSH: 3, wantPremiumSH: 0, wantLocalTB: 4.0, wantObjectTB: 0,
		},
		{
			name:         "many users need more search heads",
			in:           Subscription{IngestGBPerDay: 200, RetentionDays: 30, SearchProfile: "heavy", ConcurrentUsers: 120},
			wantIndexers: 2, wantCoreSH: 5, wantPremiumSH: 0, wantLocalTB: 7.9, wantObjectTB: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Size(tt.in, rules)
			if got.Indexers != tt.wantIndexers {
				t.Errorf("indexers = %d, want %d", got.Indexers, tt.wantIndexers)
			}
			if got.CoreSearchHeads != tt.wantCoreSH {
				t.Errorf("core search heads = %d, want %d", got.CoreSearchHeads, tt.wantCoreSH)
			}
			if got.PremiumSearchHeads != tt.wantPremiumSH {
				t.Errorf("premium search heads = %d, want %d", got.PremiumSearchHeads, tt.wantPremiumSH)
			}
			if got.LocalStorageTB != tt.wantLocalTB {
				t.Errorf("local storage = %.1f TB, want %.1f", got.LocalStorageTB, tt.wantLocalTB)
			}
			if got.ObjectStoreTB != tt.wantObjectTB {
				t.Errorf("object store = %.1f TB, want %.1f", got.ObjectStoreTB, tt.wantObjectTB)
			}
			if len(got.Explanations) == 0 {
				t.Error("expected explanations, got none")
			}
		})
	}
}

// A "property" test: more data must never mean fewer servers.
func TestMoreIngestNeverMeansFewerIndexers(t *testing.T) {
	rules := loadTestRules(t)
	prev := 0
	for gb := 10.0; gb <= 5000; gb += 10 {
		got := Size(Subscription{IngestGBPerDay: gb, RetentionDays: 30, SearchProfile: "moderate"}, rules)
		if got.Indexers < prev {
			t.Fatalf("at %.0f GB/day indexers dropped from %d to %d", gb, prev, got.Indexers)
		}
		prev = got.Indexers
	}
}

func TestValidate(t *testing.T) {
	rules := loadTestRules(t)
	good := Subscription{CustomerID: "acme", Cloud: "aws", Region: "ap-southeast-2",
		IngestGBPerDay: 100, RetentionDays: 90, SearchProfile: "moderate"}
	if errs := good.Validate(rules); len(errs) != 0 {
		t.Fatalf("valid input rejected: %v", errs)
	}
	bad := good
	bad.IngestGBPerDay = -5
	bad.Region = "mars-1"
	bad.PremiumApps = []string{"made_up"}
	if errs := bad.Validate(rules); len(errs) != 3 {
		t.Fatalf("want 3 errors, got %d: %v", len(errs), errs)
	}
}

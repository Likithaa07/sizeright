// Package sizing turns a Splunk Cloud subscription into an infrastructure
// recommendation. It does no network or database work, so it is easy to test.
package sizing

import (
	"fmt"
	"math"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// Rules holds every sizing constant. They live in rules/v1.yaml, not in code.
type Rules struct {
	Version             string             `yaml:"version"`
	IndexerCapacity     map[string]float64 `yaml:"indexer_capacity_gb_per_day"`
	MinIndexersHA       int                `yaml:"min_indexers_ha"`
	ReplicationFactor   int                `yaml:"replication_factor"`
	SearchFactor        int                `yaml:"search_factor"`
	RawdataRatio        float64            `yaml:"rawdata_ratio"`
	TsidxRatio          float64            `yaml:"tsidx_ratio"`
	Contingency         float64            `yaml:"contingency"`
	SmartStoreCacheDays int                `yaml:"smartstore_cache_days"`
	SHCMembersHA        int                `yaml:"shc_members_ha"`
	UsersPerSearchHead  int                `yaml:"users_per_search_head"`
	MaxIngestGBPerDay   float64            `yaml:"max_ingest_gb_per_day"`
	MaxRetentionDays    int                `yaml:"max_retention_days"`
}

// LoadRules reads and parses a rules file.
func LoadRules(path string) (Rules, error) {
	var r Rules
	b, err := os.ReadFile(path)
	if err != nil {
		return r, fmt.Errorf("read rules: %w", err)
	}
	if err := yaml.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("parse rules: %w", err)
	}
	return r, nil
}

// Subscription is what the customer bought.
type Subscription struct {
	CustomerID       string   `json:"customer_id"`
	Cloud            string   `json:"cloud"`
	Region           string   `json:"region"`
	IngestGBPerDay   float64  `json:"ingest_gb_per_day"`
	RetentionDays    int      `json:"retention_days"`
	SearchProfile    string   `json:"search_profile"`
	ConcurrentUsers  int      `json:"concurrent_users"`
	PremiumApps      []string `json:"premium_apps"`
	SmartStore       bool     `json:"smartstore"`
	HighAvailability bool     `json:"high_availability"`
}

// Result is the recommended architecture.
type Result struct {
	RulesVersion       string   `json:"rules_version"`
	Indexers           int      `json:"indexers"`
	CoreSearchHeads    int      `json:"core_search_heads"`
	PremiumSearchHeads int      `json:"premium_search_heads"`
	LocalStorageTB     float64  `json:"local_storage_tb"`
	ObjectStoreTB      float64  `json:"object_store_tb"`
	Explanations       []string `json:"explanations"`
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// Size applies the rules to a subscription. Call Validate first.
func Size(s Subscription, r Rules) Result {
	res := Result{RulesVersion: r.Version}
	explain := func(format string, args ...any) {
		res.Explanations = append(res.Explanations, fmt.Sprintf(format, args...))
	}
	hasES := slices.Contains(s.PremiumApps, "enterprise_security")

	// 1. Indexers
	capacity := r.IndexerCapacity[s.SearchProfile]
	if hasES {
		capacity = r.IndexerCapacity["enterprise_security"]
		explain("Enterprise Security enabled: each indexer handles %.0f GB/day", capacity)
	} else {
		explain("%s search profile: each indexer handles %.0f GB/day", s.SearchProfile, capacity)
	}
	res.Indexers = int(math.Ceil(s.IngestGBPerDay / capacity))
	if s.HighAvailability && res.Indexers < r.MinIndexersHA {
		res.Indexers = r.MinIndexersHA
		explain("High availability: raised to the minimum of %d indexers", r.MinIndexersHA)
	}

	// 2. Storage
	perDayGB := s.IngestGBPerDay * (float64(r.ReplicationFactor)*r.RawdataRatio +
		float64(r.SearchFactor)*r.TsidxRatio)
	if s.SmartStore {
		days := min(s.RetentionDays, r.SmartStoreCacheDays)
		res.LocalStorageTB = round1(perDayGB * float64(days) * r.Contingency / 1000)
		res.ObjectStoreTB = round1(s.IngestGBPerDay * (r.RawdataRatio + r.TsidxRatio) *
			float64(s.RetentionDays) / 1000)
		explain("SmartStore: %d days cached locally, full %d days in object storage", days, s.RetentionDays)
	} else {
		res.LocalStorageTB = round1(perDayGB * float64(s.RetentionDays) * r.Contingency / 1000)
		explain("Classic storage: %d days on local disk across the cluster", s.RetentionDays)
	}

	// 3. Search heads
	res.CoreSearchHeads = 1
	if s.HighAvailability {
		res.CoreSearchHeads = r.SHCMembersHA
	}
	if needed := int(math.Ceil(float64(s.ConcurrentUsers) / float64(r.UsersPerSearchHead))); needed > res.CoreSearchHeads {
		res.CoreSearchHeads = needed
	}
	explain("%d core search heads for %d concurrent users", res.CoreSearchHeads, s.ConcurrentUsers)
	if hasES {
		res.PremiumSearchHeads++
		explain("Enterprise Security gets its own dedicated search head")
	}
	return res
}

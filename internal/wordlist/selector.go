package wordlist

import (
	"strings"

	"reconx/internal/config"
)

// Tier represents the intensity level for wordlist selection.
type Tier string

const (
	TierQuick      Tier = "quick"
	TierStandard   Tier = "standard"
	TierAggressive Tier = "aggressive"
)

// Selector picks wordlists based on detected technologies and workflow tier.
type Selector struct {
	resolver  *Resolver
	wordlists config.WordlistsConfig
}

// NewSelector creates a wordlist selector.
func NewSelector(resolver *Resolver, wl config.WordlistsConfig) *Selector {
	return &Selector{resolver: resolver, wordlists: wl}
}

// DNSWordlist returns the DNS subdomain wordlist for the given tier.
func (s *Selector) DNSWordlist(tier Tier) string {
	switch tier {
	case TierQuick:
		return s.wordlists.DNSQuick
	case TierAggressive:
		return s.wordlists.DNSAggressive
	default:
		return s.wordlists.DNSStandard
	}
}

// WebWordlist returns the web content wordlist for the given tier.
func (s *Selector) WebWordlist(tier Tier) string {
	switch tier {
	case TierQuick:
		return s.wordlists.WebQuick
	case TierAggressive:
		return s.wordlists.WebAggressive
	default:
		return s.wordlists.WebStandard
	}
}

// ForTechnologies returns additional wordlists based on detected technologies.
// The techs parameter is a list of technology names from Phase 4 fingerprinting.
func (s *Selector) ForTechnologies(techs []string) []string {
	var wordlists []string
	seen := make(map[string]bool)

	for _, tech := range techs {
		lower := strings.ToLower(tech)

		// CMS detection
		if containsAny(lower, "wordpress", "wp-") {
			s.addIfExists(&wordlists, seen, s.wordlists.CMSWordpress)
		}
		if containsAny(lower, "drupal") {
			s.addIfExists(&wordlists, seen, s.wordlists.CMSDrupal)
		}
		if containsAny(lower, "joomla") {
			s.addIfExists(&wordlists, seen, s.wordlists.CMSJoomla)
		}

		// Language/framework detection
		if containsAny(lower, "php", "laravel", "symfony", "codeigniter") {
			s.addIfExists(&wordlists, seen, s.wordlists.TechPHP)
		}
		if containsAny(lower, "java", "spring", "tomcat", "struts") {
			s.addIfExists(&wordlists, seen, s.wordlists.TechJava)
		}
		if containsAny(lower, "ruby", "rails", "sinatra") {
			s.addIfExists(&wordlists, seen, s.wordlists.TechRoR)
		}
	}

	return wordlists
}

// ForSiteType returns wordlists appropriate for the site classification.
func (s *Selector) ForSiteType(siteType string, tier Tier) []string {
	var wordlists []string

	switch strings.ToLower(siteType) {
	case "api":
		wordlists = append(wordlists, s.wordlists.APIEndpoints)
		if tier == TierAggressive {
			wordlists = append(wordlists, s.wordlists.APIWild)
		}
	default:
		wordlists = append(wordlists, s.WebWordlist(tier))
	}

	return wordlists
}

// ParamWordlist returns the parameter fuzzing wordlist.
func (s *Selector) ParamWordlist() string {
	return s.wordlists.Params
}

func (s *Selector) addIfExists(wordlists *[]string, seen map[string]bool, path string) {
	if path == "" || seen[path] {
		return
	}
	if s.resolver.Exists(path) {
		*wordlists = append(*wordlists, path)
		seen[path] = true
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

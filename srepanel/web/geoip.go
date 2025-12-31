package web

import (
	"net"
	"strings"
)

// GeoLocation represents geographical information for an IP address
type GeoLocation struct {
	City        string
	Country     string
	CountryCode string
	FlagEmoji   string
	IsLocal     bool
}

// LookupIPLocation performs a GeoIP lookup for the given IP address
func (s *Server) LookupIPLocation(ipAddress string) *GeoLocation {
	// Handle localhost addresses
	if isLocalhost(ipAddress) {
		return &GeoLocation{
			City:        "localhost",
			Country:     "Local",
			CountryCode: "",
			FlagEmoji:   "🏠",
			IsLocal:     true,
		}
	}
	
	// If no GeoIP database is loaded, return nil
	if s.GeoIPDB == nil {
		return nil
	}
	
	// Parse IP address
	ip := net.ParseIP(strings.Trim(ipAddress, "[]"))
	if ip == nil {
		return nil
	}
	
	// Lookup in GeoIP database
	record, err := s.GeoIPDB.City(ip)
	if err != nil {
		return nil
	}
	
	city := ""
	if len(record.City.Names) > 0 {
		city = record.City.Names["en"]
	}
	
	country := ""
	if len(record.Country.Names) > 0 {
		country = record.Country.Names["en"]
	}
	
	countryCode := record.Country.IsoCode
	
	return &GeoLocation{
		City:        city,
		Country:     country,
		CountryCode: countryCode,
		FlagEmoji:   countryCodeToFlag(countryCode),
		IsLocal:     false,
	}
}

// isLocalhost checks if an IP address is localhost
func isLocalhost(ipAddress string) bool {
	ip := strings.Trim(ipAddress, "[]")
	return ip == "::1" || ip == "127.0.0.1" || ip == "localhost"
}

// countryCodeToFlag converts ISO country code to flag emoji
func countryCodeToFlag(code string) string {
	if code == "" {
		return ""
	}
	
	// Convert ISO country code to regional indicator symbols
	// A = 🇦 (U+1F1E6), B = 🇧 (U+1F1E7), etc.
	code = strings.ToUpper(code)
	if len(code) != 2 {
		return ""
	}
	
	flag := ""
	for _, c := range code {
		if c >= 'A' && c <= 'Z' {
			// Regional indicator symbol letter A starts at U+1F1E6
			flag += string(rune(0x1F1E6 + (c - 'A')))
		}
	}
	
	return flag
}

// formatLocation formats a GeoLocation for display
func formatLocation(loc *GeoLocation) string {
	if loc == nil {
		return "-"
	}
	
	if loc.IsLocal {
		return loc.FlagEmoji + " localhost"
	}
	
	parts := []string{}
	if loc.FlagEmoji != "" {
		parts = append(parts, loc.FlagEmoji)
	}
	if loc.City != "" {
		parts = append(parts, loc.City)
	}
	if loc.Country != "" {
		parts = append(parts, loc.Country)
	}
	
	if len(parts) == 0 {
		return "-"
	}
	
	return strings.Join(parts, " ")
}

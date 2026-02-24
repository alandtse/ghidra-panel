package web

import (
	"time"

	"github.com/oschwald/geoip2-golang"
)

// GeoIPMetadata contains basic information about the loaded database
type GeoIPMetadata struct {
	Type      string
	BuildDate time.Time
}

// GetGeoIPMetadata returns metadata about the loaded database, or nil if not loaded
func (s *Server) GetGeoIPMetadata() *GeoIPMetadata {
	if s.GeoIPDB == nil {
		return nil
	}
	m := s.GeoIPDB.Metadata()
	return &GeoIPMetadata{
		Type:      m.DatabaseType,
		BuildDate: time.Unix(int64(m.BuildEpoch), 0),
	}
}

// LoadGeoIPDatabase loads the MaxMind GeoLite2 database for IP geolocation
func (s *Server) LoadGeoIPDatabase(dbPath string) error {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return err
	}

	s.GeoIPDB = db
	return nil
}

// CloseGeoIPDatabase closes the GeoIP database
func (s *Server) CloseGeoIPDatabase() error {
	if s.GeoIPDB != nil {
		return s.GeoIPDB.Close()
	}
	return nil
}

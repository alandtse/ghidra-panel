package web

import (
	"github.com/oschwald/geoip2-golang"
)

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

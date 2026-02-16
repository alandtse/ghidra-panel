package web

import "net/http"

func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		// Set standard security headers
		wr.Header().Set("X-Frame-Options", "DENY")
		wr.Header().Set("X-Content-Type-Options", "nosniff")
		wr.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		wr.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; img-src 'self' data: https://cdn.discordapp.com; font-src 'self' https://cdnjs.cloudflare.com")

		// Strict-Transport-Security if we are using HTTPS (implied by secure cookies)
		if s.useSecureCookie {
			wr.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(wr, req)
	})
}

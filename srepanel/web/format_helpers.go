package web

import (
	"fmt"
	"time"
)

// formatUptime formats a duration into human-readable uptime
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatSize formats bytes into human-readable size (KB, MB, GB)
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDate formats Unix epoch milliseconds into human-readable date
func formatDate(millis int64) string {
	if millis == 0 {
		return "N/A"
	}
	t := time.UnixMilli(millis)
	
	// If within last 24 hours, show relative time
	now := time.Now()
	diff := now.Sub(t)
	
	if diff < 24*time.Hour {
		if diff < time.Hour {
			mins := int(diff.Minutes())
			if mins == 0 {
				return "Just now"
			}
			return fmt.Sprintf("%dm ago", mins)
		}
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	
	// If within last 7 days, show days ago
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
	
	// If within current year, show month and day
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	
	// Otherwise show full date
	return t.Format("Jan 2, 2006")
}

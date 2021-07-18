package github

// Author of Git commit
type Author struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Login     string `json:"login"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}

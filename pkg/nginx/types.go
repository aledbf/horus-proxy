package nginx

type Upstream struct {
	Address string `json:"address,omitempty"`
	Port    int    `json:"port,omitempty"`
}

// Server defines an NGINX server section
type Server struct {
	Name      string     `json:"name,omitempty"`
	Port      int        `json:"port,omitempty"`
	Upstreams []Upstream `json:"upstreams,omitempty"`
}

// Configuration defines an NGINX configuration
type Configuration struct {
	// Servers server sections
	Servers []Server `json:"servers,omitempty"`
}

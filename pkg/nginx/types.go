package nginx

type Endpoint struct {
	Address string `json:"address,omitempty"`
	Port    string `json:"port,omitempty"`
}

// Server defines an NGINX server section
type Server struct {
	Name      string     `json:"name,omitempty"`
	Port      string     `json:"port,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// Configuration defines an NGINX configuration
type Configuration struct {
	// Servers server sections
	Servers []Server `json:"servers,omitempty"`
}

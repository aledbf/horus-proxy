package nginx

// Endpoint defines the IP address and port of a pod from the deployment
type Endpoint struct {
	Address string `json:"address,omitempty"`
	Port    string `json:"port,omitempty"`
}

// Equal compares the endpoint with another one
func (e *Endpoint) Equal(to *Endpoint) bool {
	return e.Address == to.Address && e.Port == to.Port
}

// Server defines an NGINX server section
type Server struct {
	Name      string     `json:"name,omitempty"`
	Port      string     `json:"port,omitempty"`
	Endpoints []Endpoint `json:"endpoints"`
}

var compareEndpointsFunc = func(e1, e2 interface{}) bool {
	ep1, ok := e1.(Endpoint)
	if !ok {
		return false
	}

	ep2, ok := e2.(Endpoint)
	if !ok {
		return false
	}

	return (&ep1).Equal(&ep2)
}

func compareEndpoints(a, b []Endpoint) bool {
	return Compare(a, b, compareEndpointsFunc)
}

// Equal compares the server with another one
func (e *Server) Equal(to *Server) bool {
	if e.Name != to.Name {
		return false
	}

	if e.Port != to.Port {
		return false
	}

	return compareEndpoints(e.Endpoints, to.Endpoints)
}

var compareServerFunc = func(e1, e2 interface{}) bool {
	ep1, ok := e1.(Server)
	if !ok {
		return false
	}

	ep2, ok := e2.(Server)
	if !ok {
		return false
	}

	return (&ep1).Equal(&ep2)
}

func compareServers(a, b []Server) bool {
	return Compare(a, b, compareServerFunc)
}

// Configuration defines an NGINX configuration
type Configuration struct {
	// Servers server sections
	Servers []Server `json:"servers"`
}

// Equal tests for equality between two Server types
func (c *Configuration) Equal(to *Configuration) bool {
	return compareServers(c.Servers, to.Servers)
}

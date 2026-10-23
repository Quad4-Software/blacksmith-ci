package crow

import (
	"fmt"
	"net/url"
)

const (
	pathGlobalRegistries = "%s/api/v1/registries"
	pathGlobalRegistry   = "%s/api/v1/registries/%s"
)

// GlobalRegistry returns an global registry by name.
func (c *client) GlobalRegistry(registry string) (*Registry, error) {
	out := new(Registry)
	uri := fmt.Sprintf(pathGlobalRegistry, c.addr, url.PathEscape(registry))
	err := c.get(uri, out)
	return out, err
}

// GlobalRegistryList returns a list of all global registries.
func (c *client) GlobalRegistryList(opt RegistryListOptions) ([]*Registry, error) {
	var out []*Registry
	uri, _ := url.Parse(fmt.Sprintf(pathGlobalRegistries, c.addr))
	uri.RawQuery = opt.getURLQuery().Encode()
	err := c.get(uri.String(), &out)
	return out, err
}

// GlobalRegistryCreate creates a global registry.
func (c *client) GlobalRegistryCreate(in *Registry) (*Registry, error) {
	out := new(Registry)
	uri := fmt.Sprintf(pathGlobalRegistries, c.addr)
	err := c.post(uri, in, out)
	return out, err
}

// GlobalRegistryUpdate updates a global registry.
func (c *client) GlobalRegistryUpdate(in *Registry) (*Registry, error) {
	out := new(Registry)
	uri := fmt.Sprintf(pathGlobalRegistry, c.addr, url.PathEscape(in.Address))
	err := c.patch(uri, in, out)
	return out, err
}

// GlobalRegistryDelete deletes a global registry.
func (c *client) GlobalRegistryDelete(registry string) error {
	uri := fmt.Sprintf(pathGlobalRegistry, c.addr, url.PathEscape(registry))
	return c.delete(uri)
}

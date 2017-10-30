package cmds

import (
	"errors"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/config"
)

type Environment struct {
	Online     bool
	ConfigRoot string
	ReqLog     *ReqLog

	config     *config.Config
	LoadConfig func(path string) (*config.Config, error)

	node          *core.IpfsNode
	ConstructNode func() (*core.IpfsNode, error)
}

// GetConfig returns the config of the current Command exection
// rnvironment. It may load it with the providied function.
func (c *Environment) GetConfig() (*config.Config, error) {
	var err error
	if c.config == nil {
		if c.LoadConfig == nil {
			return nil, errors.New("nil LoadConfig function")
		}
		c.config, err = c.LoadConfig(c.ConfigRoot)
	}
	return c.config, err
}

// GetNode returns the node of the current Command exection
// rnvironment. It may construct it with the provided function.
func (c *Environment) GetNode() (*core.IpfsNode, error) {
	var err error
	if c.node == nil {
		if c.ConstructNode == nil {
			return nil, errors.New("nil ConstructNode function")
		}
		c.node, err = c.ConstructNode()
	}
	return c.node, err
}

// NodeWithoutConstructing returns the underlying node variable
// so that clients may close it.
func (c *Environment) NodeWithoutConstructing() *core.IpfsNode {
	return c.node
}

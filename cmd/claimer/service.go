package main

import (
	"github.com/kentquirk/boneful"
	"github.com/oneiro-ndev/rest"
	log "github.com/sirupsen/logrus"
)

type claimService struct {
	Logger *log.Entry
	Config *Config
}

var _ rest.Builder = (*claimService)(nil)

// Logger implements rest.Builder
func (c *claimService) GetLogger() *log.Entry {
	return c.Logger
}

// Build builds the service from the routes defined within
//
// path is the top-level path which gets you to this service
func (c *claimService) Build(logger *log.Entry, path string) *boneful.Service {
	c.Logger = logger

	svc := new(boneful.Service).
		Path(path).
		Doc("respond to node winner notices with `ClaimNodeReward` txs")

	svc.Route(svc.POST("/claim_winner").
		To(Claim(c.Config, c.Logger)).
		Doc("respond to a notification of node winner by sending a claim as appropriate").
		Produces("application/json").
		Writes(struct {
			Dispatched bool `json:"dispatched"`
		}{true}),
	)

	return svc
}

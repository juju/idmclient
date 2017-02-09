// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package idmtest_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon-bakery.v1/bakery"
	"gopkg.in/macaroon-bakery.v1/bakery/checkers"
	"gopkg.in/macaroon-bakery.v1/httpbakery"
	"gopkg.in/macaroon.v1"

	"github.com/juju/idmclient"
	"github.com/juju/idmclient/idmtest"
	idmparams "github.com/juju/idmclient/params"
)

type suite struct{}

var _ = gc.Suite(&suite{})

func (*suite) TestDischarge(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("bob")
	client := srv.Client("bob")
	bsvc, err := bakery.NewService(bakery.NewServiceParams{
		Locator: srv,
	})
	c.Assert(err, gc.IsNil)
	m, err := bsvc.NewMacaroon("", nil, []checkers.Caveat{{
		Location:  srv.URL.String() + "/v1/discharger",
		Condition: "is-authenticated-user",
	}})
	c.Assert(err, gc.IsNil)

	ms, err := client.DischargeAll(m)
	c.Assert(err, gc.IsNil)

	// Make sure that the macaroon discharged correctly and that it
	// has the right declared caveats.
	attrs, err := bsvc.CheckAny([]macaroon.Slice{ms}, nil, checkers.New())
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, jc.DeepEquals, map[string]string{
		"username": "bob",
	})
}

func (*suite) TestDischargeConditionWithDomain(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("bob")
	client := srv.Client("bob")
	bsvc, err := bakery.NewService(bakery.NewServiceParams{
		Locator: srv,
	})
	c.Assert(err, gc.IsNil)
	m, err := bsvc.NewMacaroon("", nil, []checkers.Caveat{{
		Location:  srv.URL.String() + "/v1/discharger",
		Condition: "is-authenticated-user @test-domain",
	}})
	c.Assert(err, gc.IsNil)

	ms, err := client.DischargeAll(m)
	c.Assert(err, gc.IsNil)

	// Make sure that the macaroon discharged correctly and that it
	// has the right declared caveats.
	attrs, err := bsvc.CheckAny([]macaroon.Slice{ms}, nil, checkers.New())
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, jc.DeepEquals, map[string]string{
		"username": "bob",
	})
}

func (*suite) TestDischargeConditionWithBadArgument(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("bob")
	client := srv.Client("bob")
	bsvc, err := bakery.NewService(bakery.NewServiceParams{
		Locator: srv,
	})
	c.Assert(err, gc.IsNil)
	m, err := bsvc.NewMacaroon("", nil, []checkers.Caveat{{
		Location:  srv.URL.String() + "/v1/discharger",
		Condition: "is-authenticated-user +test-domain",
	}})
	c.Assert(err, gc.IsNil)

	_, err = client.DischargeAll(m)
	c.Assert(err, gc.ErrorMatches, `cannot get discharge from "http://.*/v1/discharger": third party refused discharge: cannot discharge: unknown third party caveat "is-authenticated-user \+test-domain"`)
}

func (*suite) TestDischargeConditionWithBadDomain(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("bob")
	client := srv.Client("bob")
	bsvc, err := bakery.NewService(bakery.NewServiceParams{
		Locator: srv,
	})
	c.Assert(err, gc.IsNil)
	m, err := bsvc.NewMacaroon("", nil, []checkers.Caveat{{
		Location:  srv.URL.String() + "/v1/discharger",
		Condition: "is-authenticated-user @-test-domain",
	}})
	c.Assert(err, gc.IsNil)

	_, err = client.DischargeAll(m)
	c.Assert(err, gc.ErrorMatches, `cannot get discharge from "http://.*/v1/discharger": third party refused discharge: cannot discharge: invalid domain "-test-domain"`)
}

func (*suite) TestDischargeDefaultUser(c *gc.C) {
	srv := idmtest.NewServer()
	srv.SetDefaultUser("bob")

	bsvc, err := bakery.NewService(bakery.NewServiceParams{
		Locator: srv,
	})
	c.Assert(err, gc.IsNil)
	m, err := bsvc.NewMacaroon("", nil, []checkers.Caveat{{
		Location:  srv.URL.String() + "/v1/discharger",
		Condition: "is-authenticated-user",
	}})
	c.Assert(err, gc.IsNil)

	client := httpbakery.NewClient()
	ms, err := client.DischargeAll(m)
	c.Assert(err, gc.IsNil)

	// Make sure that the macaroon discharged correctly and that it
	// has the right declared caveats.
	attrs, err := bsvc.CheckAny([]macaroon.Slice{ms}, nil, checkers.New())
	c.Assert(err, gc.IsNil)
	c.Assert(attrs, jc.DeepEquals, map[string]string{
		"username": "bob",
	})
}

func (*suite) TestGroups(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("bob", "beatles", "bobbins")
	srv.AddUser("alice")

	client := idmclient.New(idmclient.NewParams{
		BaseURL: srv.URL.String(),
		Client:  srv.Client("bob"),
	})
	groups, err := client.UserGroups(&idmparams.UserGroupsRequest{
		Username: "bob",
	})
	c.Assert(err, gc.IsNil)
	c.Assert(groups, jc.DeepEquals, []string{"beatles", "bobbins"})

	groups, err = client.UserGroups(&idmparams.UserGroupsRequest{
		Username: "alice",
	})
	c.Assert(err, gc.IsNil)
	c.Assert(groups, gc.HasLen, 0)
}

func (s *suite) TestAddUserWithExistingGroups(c *gc.C) {
	srv := idmtest.NewServer()
	srv.AddUser("alice", "anteaters")
	srv.AddUser("alice")
	srv.AddUser("alice", "goof", "anteaters")

	client := idmclient.New(idmclient.NewParams{
		BaseURL: srv.URL.String(),
		Client:  srv.Client("alice"),
	})
	groups, err := client.UserGroups(&idmparams.UserGroupsRequest{
		Username: "alice",
	})
	c.Assert(err, gc.IsNil)
	c.Assert(groups, jc.DeepEquals, []string{"anteaters", "goof"})
}

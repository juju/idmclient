// Copyright 2017 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package idmtest

import (
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/checkers"
	"golang.org/x/net/context"
	"gopkg.in/errgo.v1"
)

const idmtestNamespace = "github.com/juju/idmclient/idmtest"

var checker = newChecker()

func newChecker() *checkers.Checker {
	ch := checkers.New(nil)
	ch.Namespace().Register(idmtestNamespace, "idmtest")
	ch.Register("discharge-id", idmtestNamespace, checkDischargeID)
	return ch
}

type dischargeIDKey struct{}

func contextWithDischargeID(ctx context.Context, dischargeID string) context.Context {
	return context.WithValue(ctx, dischargeIDKey{}, dischargeID)
}

func dischargeIDFromContext(ctx context.Context) string {
	dischargeID, _ := ctx.Value(dischargeIDKey{}).(string)
	return dischargeID
}

func dischargeIDCaveat(dischargeID string) checkers.Caveat {
	return checkers.Caveat{
		Condition: "discharge-id " + dischargeID,
		Namespace: idmtestNamespace,
	}
}

func checkDischargeID(ctx context.Context, cond, arg string) error {
	if dischargeIDFromContext(ctx) == arg {
		return nil
	}
	return errgo.New("incorrect discharge id")
}

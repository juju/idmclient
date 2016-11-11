// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ussodischarge_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/juju/httprequest"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	errgo "gopkg.in/errgo.v1"
	"gopkg.in/macaroon-bakery.v2-unstable/httpbakery"
	"gopkg.in/macaroon.v2-unstable"

	"github.com/juju/idmclient/params"
	"github.com/juju/idmclient/ussodischarge"
)

var _ httpbakery.Visitor = (*ussodischarge.Visitor)(nil)

type clientSuite struct {
	testMacaroon          *macaroon.Macaroon
	testDischargeMacaroon *macaroon.Macaroon
	srv                   *httptest.Server
	macaroon              *macaroon.Macaroon
}

var _ = gc.Suite(&clientSuite{})

func (s *clientSuite) SetUpSuite(c *gc.C) {
	var err error
	s.testMacaroon, err = macaroon.New([]byte("test rootkey"), []byte("test macaroon"), "test location", macaroon.LatestVersion)
	c.Assert(err, gc.Equals, nil)
	// Discharge macaroons from Ubuntu SSO will be binary encoded in the version 1 format.
	s.testDischargeMacaroon, err = macaroon.New([]byte("test discharge rootkey"), []byte("test discharge macaroon"), "test discharge location", macaroon.V1)
	c.Assert(err, gc.Equals, nil)
}

// ServeHTTP allows us to use the test suite as a handler to test the
// client methods against.
func (s *clientSuite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/macaroon":
		s.serveMacaroon(w, r)
	case "/login":
		s.serveLogin(w, r)
	case "/api/v2/tokens/discharge":
		s.serveDischarge(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *clientSuite) SetUpTest(c *gc.C) {
	s.srv = httptest.NewServer(s)
	s.macaroon = nil
}

func (s *clientSuite) TearDownTest(c *gc.C) {
	s.srv.Close()
}

func (s *clientSuite) TestMacaroon(c *gc.C) {
	s.macaroon = s.testMacaroon
	m, err := ussodischarge.Macaroon(nil, s.srv.URL+"/macaroon")
	c.Assert(err, gc.Equals, nil)
	c.Assert(m, jc.DeepEquals, s.testMacaroon)
}

func (s *clientSuite) TestMacaroonError(c *gc.C) {
	m, err := ussodischarge.Macaroon(nil, s.srv.URL+"/macaroon")
	c.Assert(m, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, `cannot get macaroon: test error`)
}

func (s *clientSuite) TestVisitor(c *gc.C) {
	v := ussodischarge.NewVisitor(func(_ *httpbakery.Client, url string) (macaroon.Slice, error) {
		c.Assert(url, gc.Equals, s.srv.URL+"/login")
		return macaroon.Slice{s.testMacaroon}, nil
	})
	u, err := url.Parse(s.srv.URL + "/login")
	c.Assert(err, gc.Equals, nil)
	client := httpbakery.NewClient()
	err = v.VisitWebPage(client, map[string]*url.URL{ussodischarge.ProtocolName: u})
	c.Assert(err, gc.Equals, nil)
}

func (s *clientSuite) TestVisitorMethodNotSupported(c *gc.C) {
	v := ussodischarge.NewVisitor(func(_ *httpbakery.Client, url string) (macaroon.Slice, error) {
		return nil, errgo.New("function called unexpectedly")
	})
	client := httpbakery.NewClient()
	err := v.VisitWebPage(client, map[string]*url.URL{})
	c.Assert(errgo.Cause(err), gc.Equals, httpbakery.ErrMethodNotSupported)
}

func (s *clientSuite) TestVisitorFunctionError(c *gc.C) {
	v := ussodischarge.NewVisitor(func(_ *httpbakery.Client, url string) (macaroon.Slice, error) {
		return nil, errgo.WithCausef(nil, testCause, "test error")
	})
	u, err := url.Parse(s.srv.URL + "/login")
	c.Assert(err, gc.Equals, nil)
	client := httpbakery.NewClient()
	err = v.VisitWebPage(client, map[string]*url.URL{ussodischarge.ProtocolName: u})
	c.Assert(errgo.Cause(err), gc.Equals, testCause)
	c.Assert(err, gc.ErrorMatches, "test error")
}

func (s *clientSuite) TestAcquireDischarge(c *gc.C) {
	d := &ussodischarge.Discharger{
		Email:    "user@example.com",
		Password: "secret",
		OTP:      "123456",
	}
	m, err := d.AcquireDischarge(macaroon.Caveat{
		Location: s.srv.URL,
		Id:       []byte("test caveat id"),
	})
	c.Assert(err, gc.Equals, nil)
	c.Assert(m, jc.DeepEquals, s.testDischargeMacaroon)
}

func (s *clientSuite) TestAcquireDischargeError(c *gc.C) {
	d := &ussodischarge.Discharger{
		Email:    "user@example.com",
		Password: "bad-secret",
		OTP:      "123456",
	}
	m, err := d.AcquireDischarge(macaroon.Caveat{
		Location: s.srv.URL,
		Id:       []byte("test caveat id"),
	})
	c.Assert(err, gc.ErrorMatches, `Provided email/password is not correct.`)
	c.Assert(m, gc.IsNil)
}

func (s *clientSuite) TestDischargeAll(c *gc.C) {
	m := *s.testMacaroon
	err := m.AddThirdPartyCaveat([]byte("third party root key"), []byte("third party caveat id"), s.srv.URL)
	c.Assert(err, gc.Equals, nil)
	d := &ussodischarge.Discharger{
		Email:    "user@example.com",
		Password: "secret",
		OTP:      "123456",
	}
	ms, err := d.DischargeAll(&m)
	c.Assert(err, gc.Equals, nil)
	md := *s.testDischargeMacaroon
	md.Bind(m.Signature())
	c.Assert(ms, jc.DeepEquals, macaroon.Slice{&m, &md})
}

func (s *clientSuite) TestDischargeAllError(c *gc.C) {
	m := *s.testMacaroon
	err := m.AddThirdPartyCaveat([]byte("third party root key"), []byte("third party caveat id"), s.srv.URL)
	c.Assert(err, gc.Equals, nil)
	d := &ussodischarge.Discharger{
		Email:    "user@example.com",
		Password: "bad-secret",
		OTP:      "123456",
	}
	ms, err := d.DischargeAll(&m)
	c.Assert(err, gc.ErrorMatches, `cannot get discharge from ".*": Provided email/password is not correct.`)
	c.Assert(ms, gc.IsNil)
}

func (s *clientSuite) serveMacaroon(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, r, errgo.Newf("bad method: %s", r.Method))
	}
	if s.macaroon != nil {
		httprequest.WriteJSON(w, http.StatusOK, ussodischarge.MacaroonResponse{
			Macaroon: s.macaroon,
		})
	} else {
		httprequest.WriteJSON(w, http.StatusInternalServerError, params.Error{
			Message: "test error",
		})
	}
}

func (s *clientSuite) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, r, errgo.Newf("bad method: %s", r.Method))
	}
	var lr ussodischarge.LoginRequest
	if err := httprequest.Unmarshal(httprequest.Params{Request: r, Response: w}, &lr); err != nil {
		fail(w, r, err)
	}
	if n := len(lr.Login.Macaroons); n != 1 {
		fail(w, r, errgo.Newf("macaroon slice has unexpected length %d", n))
	}
	if id := lr.Login.Macaroons[0].Id(); string(id) != "test macaroon" {
		fail(w, r, errgo.Newf("unexpected macaroon sent %q", string(id)))
	}
	w.WriteHeader(http.StatusOK)
}

func (s *clientSuite) serveDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, r, errgo.Newf("bad method: %s", r.Method))
	}
	var dr ussodischarge.USSODischargeRequest
	if err := httprequest.Unmarshal(httprequest.Params{Request: r, Response: w}, &dr); err != nil {
		fail(w, r, err)
	}
	if dr.Discharge.Email == "" {
		fail(w, r, errgo.New("email not specified"))
	}
	if dr.Discharge.Password == "" {
		fail(w, r, errgo.New("password not specified"))
	}
	if dr.Discharge.OTP == "" {
		fail(w, r, errgo.New("otp not specified"))
	}
	if dr.Discharge.CaveatID == "" {
		fail(w, r, errgo.New("caveat_id not specified"))
	}
	if dr.Discharge.Email != "user@example.com" || dr.Discharge.Password != "secret" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error_list": [{"message": "Provided email/password is not correct.", "code": "invalid-credentials"}], "message": "Provided email/password is not correct.", "code": "INVALID_CREDENTIALS", "extra": {}}`))
		return
	}
	var m ussodischarge.USSOMacaroon
	m.Macaroon = *s.testDischargeMacaroon
	httprequest.WriteJSON(w, http.StatusOK, map[string]interface{}{"discharge_macaroon": &m})
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	httprequest.WriteJSON(w, http.StatusBadRequest, params.Error{
		Message: err.Error(),
	})
}

var testCause = errgo.New("test cause")
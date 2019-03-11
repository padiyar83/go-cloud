// Copyright 2019 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mysql provides functions to open MySQL databases with OpenCensus instrumentation.
package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"

	"github.com/go-sql-driver/mysql"

	"contrib.go.opencensus.io/integrations/ocsql"
)

// Scheme is the URL scheme this package registers its URLOpener under on
// DefaultMux.
const Scheme = "mysql"

func init() {
	DefaultURLMux().RegisterMySQL(Scheme, &URLOpener{})
}

// URLOpener opens URLs like "mysql://" by using the underlying MySQL driver.
// See https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters for details.
type URLOpener struct{}

// OpenMySQLURL opens a new database connection wrapped with OpenCensus instrumentation.
func (*URLOpener) OpenMySQLURL(ctx context.Context, u *url.URL) (*sql.DB, error) {
	db, err := openWithUrl(u)
	return db, err
}

func openWithUrl(url *url.URL) (*sql.DB, error) {
	return sql.OpenDB(connector{dsn: url.String()}), nil
}

type connector struct {
	dsn string
}

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.Driver().Open(c.dsn)
}

func (c connector) Driver() driver.Driver {
	return ocsql.Wrap(mysql.MySQLDriver{})
}

// A type that implements MySQLURLOpener can open connection based on a URL.
// The opener must not modify the URL argument. OpenMySQLURL must be safe to
// call from multiple goroutines.
//
// This interface is generally implemented by types in driver packages.
type MySQLURLOpener interface {
	OpenMySQLURL(ctx context.Context, u *url.URL) (*sql.DB, error)
}

// URLMux is a URL opener multiplexer. It matches the scheme of the URLs
// against a set of registered schemes and calls the opener that matches the
// URL's scheme.
//
// The zero value is a multiplexer with no registered schemes.
type URLMux struct {
	schemes map[string]MySQLURLOpener
}

// RegisterMySQL registers the opener with the given scheme. If an opener
// already exists for the scheme, RegisterMySQL panics.
func (mux *URLMux) RegisterMySQL(scheme string, opener MySQLURLOpener) {
	if mux.schemes == nil {
		mux.schemes = make(map[string]MySQLURLOpener)
	} else if _, exists := mux.schemes[scheme]; exists {
		panic(fmt.Errorf("scheme %q already registered on mux", scheme))
	}
	mux.schemes[scheme] = opener
}

// OpenMySQL calls OpenMySQLURL with the URL parsed from urlstr.
// OpenMySQL is safe to call from multiple goroutines.
func (mux *URLMux) OpenMySQL(ctx context.Context, urlstr string) (*sql.DB, error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		return nil, fmt.Errorf("open connection: %v", err)
	}
	return mux.OpenMySQLURL(ctx, u)
}

// OpenMySQLURL dispatches the URL to the opener that is registered with the
// URL's scheme. OpenMySQLURL is safe to call from multiple goroutines.
func (mux *URLMux) OpenMySQLURL(ctx context.Context, u *url.URL) (*sql.DB, error) {
	if u.Scheme == "" {
		return nil, fmt.Errorf("open connection %q: no scheme in URL", u)
	}
	var opener MySQLURLOpener
	if mux != nil {
		opener = mux.schemes[u.Scheme]
	}
	if opener == nil {
		return nil, fmt.Errorf("open connection %q: no provider registered for %s", u, u.Scheme)
	}
	return opener.OpenMySQLURL(ctx, u)
}

var defaultURLMux = new(URLMux)

// DefaultURLMux returns the URLMux used by OpenMySql.
//
// Driver packages can use this to register their MySQLURLOpener on the mux.
func DefaultURLMux() *URLMux {
	return defaultURLMux
}

// Open opens the bucket identified by the URL given. URL openers must be
// registered in the DefaultURLMux, which is typically done in driver
// packages' initialization.
//
// See the URLOpener documentation in provider-specific subpackages for more
// details on supported scheme(s) and URL parameter(s).
func Open(ctx context.Context, urlstr string) (*sql.DB, error) {
	return defaultURLMux.OpenMySQL(ctx, urlstr)
}
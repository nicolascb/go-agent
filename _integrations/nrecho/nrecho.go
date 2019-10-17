// Package nrecho instruments https://github.com/labstack/echo applications.
//
// Use this package to instrument inbound requests handled by an echo.Echo
// instance.
//
//	e := echo.New()
//	// Add the nrecho middleware before other middlewares or routes:
//	e.Use(nrecho.Middleware(app))
//
// Example: https://github.com/newrelic/go-agent/tree/master/_integrations/nrecho/example/main.go
package nrecho

import (
	"net/http"
	"reflect"

	"github.com/labstack/echo"
	newrelic "github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/internal"
)

func init() { internal.TrackUsage("integration", "framework", "echo") }

// FromContext returns the Transaction from the context if present, and nil
// otherwise.
func FromContext(c echo.Context) *newrelic.Transaction {
	return newrelic.FromContext(c.Request().Context())
}

func handlerPointer(handler echo.HandlerFunc) uintptr {
	return reflect.ValueOf(handler).Pointer()
}

func transactionName(c echo.Context) string {
	ptr := handlerPointer(c.Handler())
	if ptr == handlerPointer(echo.NotFoundHandler) {
		return "NotFoundHandler"
	}
	if ptr == handlerPointer(echo.MethodNotAllowedHandler) {
		return "MethodNotAllowedHandler"
	}
	return c.Path()
}

// Middleware creates Echo middleware that instruments requests.
//
//	e := echo.New()
//	// Add the nrecho middleware before other middlewares or routes:
//	e.Use(nrecho.Middleware(app))
//
func Middleware(app *newrelic.Application) func(echo.HandlerFunc) echo.HandlerFunc {

	if nil == app {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			rw := c.Response().Writer
			txn := app.StartTransaction(transactionName(c))
			defer txn.End()

			txn.SetWebRequest(newrelic.NewWebRequest(c.Request()))

			c.Response().Writer = txn.SetWebResponse(rw)

			// Add txn to c.Request().Context()
			c.SetRequest(c.Request().WithContext(newrelic.NewContext(c.Request().Context(), txn)))

			err = next(c)

			// Record the response code. The response headers are not captured
			// in this case because they are set after this middleware returns.
			// Designed to mimic the logic in echo.DefaultHTTPErrorHandler.
			if nil != err && !c.Response().Committed {

				c.Response().Writer = rw

				if httperr, ok := err.(*echo.HTTPError); ok {
					txn.SetWebResponse(nil).WriteHeader(httperr.Code)
				} else {
					txn.SetWebResponse(nil).WriteHeader(http.StatusInternalServerError)
				}
			}

			return
		}
	}
}

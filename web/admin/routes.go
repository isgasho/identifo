package admin

import (
	"github.com/urfave/negroni"
)

// Setup all routes.
func (ar *Router) initRoutes() {
	// do nothing on empty router (or should panic?)
	if ar.router == nil {
		return
	}

	ar.router.Path("/{login:login\\/?}").Handler(negroni.New(
		negroni.WrapFunc(ar.Login()),
	)).Methods("POST")

	ar.router.Path("/{logout:logout\\/?}").Handler(negroni.New(
		negroni.WrapFunc(ar.Logout()),
	)).Methods("POST")

	ar.router.Path("/{users:users\\/?}").Handler(negroni.New(
		ar.Session(),
		negroni.WrapFunc(ar.FetchUsers()),
	)).Methods("GET")

	users := ar.router.PathPrefix("/users").Subrouter()
	ar.router.PathPrefix("/users").Handler(negroni.New(
		ar.Session(),
		negroni.Wrap(users),
	))
	users.Path("/{id:[a-zA-Z0-9]+}").HandlerFunc(ar.DeleteUser()).Methods("DELETE")
}

package mgo

import (
	"github.com/madappgang/identifo/model"
	"github.com/madappgang/identifo/server"
)

// NewServer creates new backend service with MongoDB support.
func NewServer(settings model.ServerSettings, options ...func(*server.Server) error) (model.Server, error) {
	dbComposer, err := NewComposer(settings)
	if err != nil {
		return nil, err
	}
	return server.NewServer(settings, dbComposer, nil, options...)
}

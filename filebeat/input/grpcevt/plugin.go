package grpcevt

import (
	input "github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/feature"
)

const grpcPluginName = "grpc"

func Plugin() input.Plugin {
	return input.Plugin{
		Name:       grpcPluginName,
		Stability:  feature.Experimental,
		Deprecated: false,
		Info:       "GRPC Bulk request",
		Doc:        "Use ES bulk API to receive events from Elastic Agent",
		Manager:    input.ConfigureWith(configure),
	}
}

func configure(cfg *common.Config) (input.Input, error) {
	settings, err := readSettings(cfg)
	if err != nil {
		return nil, err
	}

	return newGRPCInput(settings)
}

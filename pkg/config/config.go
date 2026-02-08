package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	DomainField = field.StringField(
		"domain",
		field.WithDisplayName("Domain"),
		field.WithDescription("The domain for your Twingate account. ($BATON_DOMAIN)"),
		field.WithRequired(true),
	)
	ApiKeyField = field.StringField(
		"api-key",
		field.WithDisplayName("API Key"),
		field.WithDescription("The api key for your Twingate account. ($BATON_API_KEY)"),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)
	BaseURLField = field.StringField(
		"base-url",
		field.WithDescription("Override the Twingate API URL (for testing). ($BATON_BASE_URL)"),
	)

	ConfigurationFields = []field.SchemaField{
		DomainField,
		ApiKeyField,
		BaseURLField,
	}

	Configuration      = field.NewConfiguration(ConfigurationFields)
	FieldRelationships = []field.SchemaFieldRelationship{}
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConstraints(FieldRelationships...),
	field.WithConnectorDisplayName("Twingate V2"),
	field.WithHelpUrl("/docs/baton/twingate-v2"),
	field.WithIconUrl("/static/app-icons/twingate.svg"),
)

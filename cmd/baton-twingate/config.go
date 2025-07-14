package main

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

	configurationFields = []field.SchemaField{
		DomainField,
		ApiKeyField,
	}
	Configuration = field.NewConfiguration(configurationFields)
)

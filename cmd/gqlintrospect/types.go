package main

type Directive struct {
	Name         string              `json:"name"`
	Description  *string             `json:"description,omitempty"`
	Locations    []DirectiveLocation `json:"locations"`
	Args         []InputValue        `json:"args"`
	IsRepeatable bool                `json:"isRepeatable"`
}

type DirectiveLocation string

const (
	DirectiveLocationQuery                DirectiveLocation = "QUERY"
	DirectiveLocationMutation             DirectiveLocation = "MUTATION"
	DirectiveLocationSubscription         DirectiveLocation = "SUBSCRIPTION"
	DirectiveLocationField                DirectiveLocation = "FIELD"
	DirectiveLocationFragmentDefinition   DirectiveLocation = "FRAGMENT_DEFINITION"
	DirectiveLocationFragmentSpread       DirectiveLocation = "FRAGMENT_SPREAD"
	DirectiveLocationInlineFragment       DirectiveLocation = "INLINE_FRAGMENT"
	DirectiveLocationVariableDefinition   DirectiveLocation = "VARIABLE_DEFINITION"
	DirectiveLocationSchema               DirectiveLocation = "SCHEMA"
	DirectiveLocationScalar               DirectiveLocation = "SCALAR"
	DirectiveLocationObject               DirectiveLocation = "OBJECT"
	DirectiveLocationFieldDefinition      DirectiveLocation = "FIELD_DEFINITION"
	DirectiveLocationArgumentDefinition   DirectiveLocation = "ARGUMENT_DEFINITION"
	DirectiveLocationInterface            DirectiveLocation = "INTERFACE"
	DirectiveLocationUnion                DirectiveLocation = "UNION"
	DirectiveLocationEnum                 DirectiveLocation = "ENUM"
	DirectiveLocationEnumValue            DirectiveLocation = "ENUM_VALUE"
	DirectiveLocationInputObject          DirectiveLocation = "INPUT_OBJECT"
	DirectiveLocationInputFieldDefinition DirectiveLocation = "INPUT_FIELD_DEFINITION"
)

type EnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description,omitempty"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason,omitempty"`
}

type Field struct {
	Name              string       `json:"name"`
	Description       *string      `json:"description,omitempty"`
	Args              []InputValue `json:"args"`
	Type              *Type        `json:"type"`
	IsDeprecated      bool         `json:"isDeprecated"`
	DeprecationReason *string      `json:"deprecationReason,omitempty"`
}

type InputValue struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	Type         *Type   `json:"type"`
	DefaultValue *string `json:"defaultValue,omitempty"`
}

type Schema struct {
	Types            []Type      `json:"types"`
	QueryType        *Type       `json:"queryType"`
	MutationType     *Type       `json:"mutationType,omitempty"`
	SubscriptionType *Type       `json:"subscriptionType,omitempty"`
	Directives       []Directive `json:"directives"`
}

type Type struct {
	Kind          TypeKind     `json:"kind"`
	Name          *string      `json:"name,omitempty"`
	Description   *string      `json:"description,omitempty"`
	Fields        []Field      `json:"fields,omitempty"`
	Interfaces    []Type       `json:"interfaces,omitempty"`
	PossibleTypes []Type       `json:"possibleTypes,omitempty"`
	EnumValues    []EnumValue  `json:"enumValues,omitempty"`
	InputFields   []InputValue `json:"inputFields,omitempty"`
	OfType        *Type        `json:"ofType,omitempty"`
}

type TypeKind string

const (
	TypeKindScalar      TypeKind = "SCALAR"
	TypeKindObject      TypeKind = "OBJECT"
	TypeKindInterface   TypeKind = "INTERFACE"
	TypeKindUnion       TypeKind = "UNION"
	TypeKindEnum        TypeKind = "ENUM"
	TypeKindInputObject TypeKind = "INPUT_OBJECT"
	TypeKindList        TypeKind = "LIST"
	TypeKindNonNull     TypeKind = "NON_NULL"
)

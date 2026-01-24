package dto

type CreateOrganizationInput struct {
	Body struct {
		SetAsActiveOrganization bool    `json:"set_as_active_organization"`
		Name                    string  `json:"name" minLength:"1" maxLength:"100"`
		Logo                    *string `json:"logo,omitempty" format:"uri"`
	}
}

type CreateOrganizationOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

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

type Organization struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Logo      *string `json:"logo"`
	Role      string  `json:"role"`
	CreatedAt string  `json:"created_at"`
}

type ListOrganizationsOutput struct {
	Body struct {
		Organizations []Organization `json:"organizations"`
	}
}

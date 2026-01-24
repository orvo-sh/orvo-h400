package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/organizationservice"
	"github.com/orvo-sh/orvo/internal/http/helper"
	"github.com/orvo-sh/orvo/internal/http/middleware/bodyparser"
)

type organizationHttpHandler struct {
	orgService  organizationservice.Service
	authService authservice.Service
}

// Request types
type (
	createOrganizationRequest struct {
		Body struct {
			Name string  `json:"name"`
			Slug string  `json:"slug"`
			Logo *string `json:"logo,omitempty"`
		} `in:"body=json"`
	}

	updateOrganizationRequest struct {
		Body struct {
			Name *string `json:"name,omitempty"`
			Slug *string `json:"slug,omitempty"`
			Logo *string `json:"logo,omitempty"`
		} `in:"body=json"`
	}

	checkSlugRequest struct {
		Body struct {
			Slug string `json:"slug"`
		} `in:"body=json"`
	}

	inviteMemberRequest struct {
		Body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `in:"body=json"`
	}

	addMemberRequest struct {
		Body struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		} `in:"body=json"`
	}

	updateMemberRoleRequest struct {
		Body struct {
			Role string `json:"role"`
		} `in:"body=json"`
	}
)

// Response types
type OrganizationResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Logo      *string `json:"logo,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type MemberResponse struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	UserID         string  `json:"user_id"`
	Role           string  `json:"role"`
	Email          *string `json:"email,omitempty"`
	Name           *string `json:"name,omitempty"`
	Image          *string `json:"image,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type InvitationResponse struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Email          string  `json:"email"`
	Role           string  `json:"role"`
	Status         string  `json:"status"`
	ExpiresAt      string  `json:"expires_at"`
	InviterName    *string `json:"inviter_name,omitempty"`
	InviterEmail   *string `json:"inviter_email,omitempty"`
	OrgName        *string `json:"org_name,omitempty"`
	OrgSlug        *string `json:"org_slug,omitempty"`
}

func SetupOrganizationHttpHandler(r chi.Router, orgService organizationservice.Service, authService authservice.Service) {
	h := organizationHttpHandler{
		orgService:  orgService,
		authService: authService,
	}

	// All organization endpoints require authentication
	r.Route("/organizations", func(r chi.Router) {
		r.Use(h.requireSession)

		// Organization CRUD
		r.With(bodyparser.New[createOrganizationRequest]()).Post("/", h.create)
		r.With(bodyparser.New[checkSlugRequest]()).Post("/check-slug", h.checkSlug)
		r.Get("/", h.list) // List user's organizations

		r.Route("/{orgId}", func(r chi.Router) {
			r.Get("/", h.get)
			r.With(bodyparser.New[updateOrganizationRequest]()).Patch("/", h.update)
			r.Delete("/", h.delete)

			// Member management
			r.Route("/members", func(r chi.Router) {
				r.Get("/", h.listMembers)
				r.With(bodyparser.New[addMemberRequest]()).Post("/", h.addMember)
				r.Route("/{memberId}", func(r chi.Router) {
					r.Delete("/", h.removeMember)
					r.With(bodyparser.New[updateMemberRoleRequest]()).Patch("/role", h.updateMemberRole)
				})
			})

			// Invitations
			r.Route("/invitations", func(r chi.Router) {
				r.Get("/", h.listInvitations)
				r.With(bodyparser.New[inviteMemberRequest]()).Post("/", h.createInvitation)
				r.Delete("/{invitationId}", h.cancelInvitation)
			})
		})

		// Leave organization
		r.Post("/{orgId}/leave", h.leave)
	})

	// User invitations (not org-specific)
	r.Route("/invitations", func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/", h.listUserInvitations)
		r.Post("/{invitationId}/accept", h.acceptInvitation)
		r.Post("/{invitationId}/reject", h.rejectInvitation)
	})
}

// create creates a new organization
func (h *organizationHttpHandler) create(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[createOrganizationRequest](r).Body
	session := MustGetSessionFromContext(r)

	org, err := h.orgService.Create(r.Context(), organizationservice.CreateInput{
		Name:          body.Name,
		Slug:          body.Slug,
		Logo:          body.Logo,
		CreatorUserID: session.User.ID,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toOrganizationResponse(org), nil)
}

// checkSlug checks if a slug is available
func (h *organizationHttpHandler) checkSlug(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[checkSlugRequest](r).Body

	exists, err := h.orgService.CheckSlugExists(r.Context(), body.Slug)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"exists": exists}, nil)
}

// list returns all organizations the user is a member of
func (h *organizationHttpHandler) list(w http.ResponseWriter, r *http.Request) {
	session := MustGetSessionFromContext(r)

	orgs, err := h.orgService.ListByUserID(r.Context(), session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	result := make([]*OrganizationResponse, len(orgs))
	for i, org := range orgs {
		result[i] = toOrganizationResponse(org)
	}
	helper.Resp(w, result, nil)
}

// get returns a specific organization
func (h *organizationHttpHandler) get(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	org, err := h.orgService.GetByID(r.Context(), orgID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toOrganizationResponse(org), nil)
}

// update updates an organization
func (h *organizationHttpHandler) update(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	body := bodyparser.GetBodyFromContext[updateOrganizationRequest](r).Body
	session := MustGetSessionFromContext(r)

	org, err := h.orgService.Update(r.Context(), organizationservice.UpdateInput{
		OrganizationID: orgID,
		ActorUserID:    session.User.ID,
		Name:           body.Name,
		Slug:           body.Slug,
		Logo:           body.Logo,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toOrganizationResponse(org), nil)
}

// delete deletes an organization
func (h *organizationHttpHandler) delete(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	session := MustGetSessionFromContext(r)

	err := h.orgService.Delete(r.Context(), orgID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// listMembers lists all members of an organization
func (h *organizationHttpHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	members, err := h.orgService.ListMembers(r.Context(), orgID, 100, 0)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	result := make([]*MemberResponse, len(members))
	for i, m := range members {
		result[i] = toMemberResponse(m)
	}
	helper.Resp(w, result, nil)
}

// addMember adds a member directly (without invitation)
func (h *organizationHttpHandler) addMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	body := bodyparser.GetBodyFromContext[addMemberRequest](r).Body
	session := MustGetSessionFromContext(r)

	member, err := h.orgService.AddMember(r.Context(), organizationservice.AddMemberInput{
		OrganizationID: orgID,
		UserID:         body.UserID,
		Role:           body.Role,
		ActorUserID:    session.User.ID,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toMemberResponse(member), nil)
}

// removeMember removes a member from an organization
func (h *organizationHttpHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	memberID := chi.URLParam(r, "memberId")
	session := MustGetSessionFromContext(r)

	err := h.orgService.RemoveMember(r.Context(), orgID, memberID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// updateMemberRole updates a member's role
func (h *organizationHttpHandler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	memberID := chi.URLParam(r, "memberId")
	body := bodyparser.GetBodyFromContext[updateMemberRoleRequest](r).Body
	session := MustGetSessionFromContext(r)

	member, err := h.orgService.UpdateMemberRole(r.Context(), organizationservice.UpdateMemberRoleInput{
		OrganizationID: orgID,
		MemberID:       memberID,
		NewRole:        body.Role,
		ActorUserID:    session.User.ID,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toMemberResponse(member), nil)
}

// leave removes the current user from an organization
func (h *organizationHttpHandler) leave(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	session := MustGetSessionFromContext(r)

	err := h.orgService.Leave(r.Context(), orgID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// listInvitations lists all invitations for an organization
func (h *organizationHttpHandler) listInvitations(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	invitations, err := h.orgService.ListInvitations(r.Context(), orgID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	result := make([]*InvitationResponse, len(invitations))
	for i, inv := range invitations {
		result[i] = toInvitationResponse(inv)
	}
	helper.Resp(w, result, nil)
}

// createInvitation creates an invitation to join an organization
func (h *organizationHttpHandler) createInvitation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	body := bodyparser.GetBodyFromContext[inviteMemberRequest](r).Body
	session := MustGetSessionFromContext(r)

	inv, err := h.orgService.CreateInvitation(r.Context(), organizationservice.CreateInvitationInput{
		OrganizationID: orgID,
		Email:          body.Email,
		Role:           body.Role,
		InviterUserID:  session.User.ID,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toInvitationResponse(inv), nil)
}

// cancelInvitation cancels an invitation
func (h *organizationHttpHandler) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "invitationId")
	session := MustGetSessionFromContext(r)

	err := h.orgService.CancelInvitation(r.Context(), invitationID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// listUserInvitations lists all invitations for the current user
func (h *organizationHttpHandler) listUserInvitations(w http.ResponseWriter, r *http.Request) {
	session := MustGetSessionFromContext(r)

	invitations, err := h.orgService.ListUserInvitations(r.Context(), session.User.Email)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	result := make([]*InvitationResponse, len(invitations))
	for i, inv := range invitations {
		result[i] = toInvitationResponse(inv)
	}
	helper.Resp(w, result, nil)
}

// acceptInvitation accepts an invitation
func (h *organizationHttpHandler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "invitationId")
	session := MustGetSessionFromContext(r)

	member, err := h.orgService.AcceptInvitation(r.Context(), invitationID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toMemberResponse(member), nil)
}

// rejectInvitation rejects an invitation
func (h *organizationHttpHandler) rejectInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID := chi.URLParam(r, "invitationId")
	session := MustGetSessionFromContext(r)

	err := h.orgService.RejectInvitation(r.Context(), invitationID, session.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// requireSession middleware
func (h *organizationHttpHandler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if token == "" {
			helper.Resp(w, nil, authservice.ErrSessionNotFound)
			return
		}

		sessionData, err := h.authService.GetSession(r.Context(), token)
		if err != nil {
			helper.Resp(w, nil, err)
			return
		}

		ctx := setSessionInContext(r.Context(), sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper functions
func toOrganizationResponse(org *organizationservice.Organization) *OrganizationResponse {
	return &OrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Logo:      org.Logo,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toMemberResponse(m *organizationservice.Member) *MemberResponse {
	return &MemberResponse{
		ID:             m.ID,
		OrganizationID: m.OrganizationID,
		UserID:         m.UserID,
		Role:           m.Role,
		Email:          m.Email,
		Name:           m.Name,
		Image:          m.Image,
		CreatedAt:      m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toInvitationResponse(inv *organizationservice.Invitation) *InvitationResponse {
	return &InvitationResponse{
		ID:             inv.ID,
		OrganizationID: inv.OrganizationID,
		Email:          inv.Email,
		Role:           inv.Role,
		Status:         inv.Status,
		ExpiresAt:      inv.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		InviterName:    inv.InviterName,
		InviterEmail:   inv.InviterEmail,
		OrgName:        inv.OrgName,
		OrgSlug:        inv.OrgSlug,
	}
}

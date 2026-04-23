// 遵循project_guide.md
package searchprojection

import (
	"context"
	"strings"
	"testing"
)

// TestNoopProjector_RequiresMandatoryFields locks down the validation
// contract every Projector implementation must honour: company scope,
// entity identity, and minimum displayability. Phase 1's EntProjector
// will reuse the same checks.
func TestNoopProjector_RequiresMandatoryFields(t *testing.T) {
	cases := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{
			name:    "missing company",
			doc:     Document{EntityType: "invoice", EntityID: 1, Title: "x", URLPath: "/x"},
			wantErr: "CompanyID",
		},
		{
			name:    "missing entity type",
			doc:     Document{CompanyID: 1, EntityID: 1, Title: "x", URLPath: "/x"},
			wantErr: "EntityType",
		},
		{
			name:    "missing entity id",
			doc:     Document{CompanyID: 1, EntityType: "invoice", Title: "x", URLPath: "/x"},
			wantErr: "EntityID",
		},
		{
			name:    "missing title",
			doc:     Document{CompanyID: 1, EntityType: "invoice", EntityID: 1, URLPath: "/x"},
			wantErr: "Title",
		},
		{
			name:    "missing url",
			doc:     Document{CompanyID: 1, EntityType: "invoice", EntityID: 1, Title: "x"},
			wantErr: "URLPath",
		},
	}
	p := NoopProjector{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := p.Upsert(context.Background(), tc.doc)
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestNoopProjector_AcceptsValidDocument(t *testing.T) {
	p := NoopProjector{}
	err := p.Upsert(context.Background(), Document{
		CompanyID:  1,
		EntityType: "customer",
		EntityID:   42,
		Title:      "Acme Corp",
		URLPath:    "/customers/42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopProjector_DeleteValidation(t *testing.T) {
	p := NoopProjector{}
	if err := p.Delete(context.Background(), 0, "invoice", 1); err == nil {
		t.Error("expected error for zero companyID")
	}
	if err := p.Delete(context.Background(), 1, "", 1); err == nil {
		t.Error("expected error for empty entityType")
	}
	if err := p.Delete(context.Background(), 1, "invoice", 0); err == nil {
		t.Error("expected error for zero entityID")
	}
	if err := p.Delete(context.Background(), 1, "invoice", 1); err != nil {
		t.Errorf("unexpected error on valid delete: %v", err)
	}
}

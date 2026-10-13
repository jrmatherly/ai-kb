package tools

import (
	"context"
	"encoding/json"
	"testing"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/types"
)

type rejectingWikiService struct{ sourceRefWikiService }

func (*rejectingWikiService) CreatePage(context.Context, *types.WikiPage) (*types.WikiPage, error) {
	return nil, apperrors.NewNotFoundError(`wiki page references unknown folder "nope"`)
}

// The model reads a service error as plain text, without the AppError code prefix.
func TestWikiWritePageRendersServiceErrorAsMessage(t *testing.T) {
	tool := NewWikiWritePageTool(&rejectingWikiService{}, []string{"kb-1"}, nil, NewWikiRouteResolver())
	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept"}`))
	if err != nil || result == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if want := `Failed to create page: wiki page references unknown folder "nope"`; result.Error != want {
		t.Fatalf("error = %q, want %q", result.Error, want)
	}
}

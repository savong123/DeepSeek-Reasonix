package dispatch

import (
	"errors"
	"slices"

	"reasonix/internal/ext/extension"
)

type LiveCompactionContext struct {
	Floor       int  `json:"floor"`
	LastUser    int  `json:"lastUser"`
	InputTokens int  `json:"inputTokens"`
	Growth      int  `json:"growth"`
	Cloud       bool `json:"cloud"`
	Drain       bool `json:"drain"`
}

type LiveCompactionCandidate struct {
	Start       int  `json:"start"`
	End         int  `json:"end"`
	Tokens      int  `json:"tokens"`
	ToolBatches int  `json:"toolBatches"`
	LongRead    bool `json:"longRead,omitempty"`
	Pointer     bool `json:"pointer,omitempty"`
	Failure     bool `json:"failure,omitempty"`
}

type LiveCompactionCut struct {
	Start *int `json:"start"`
	End   *int `json:"end"`
}

type LiveCompactionPayload struct {
	Context    *LiveCompactionContext    `json:"context,omitempty"`
	Candidates []LiveCompactionCandidate `json:"candidates,omitempty"`
	Cut        *LiveCompactionCut        `json:"cut,omitempty"`
}

func (LiveCompactionPayload) Point() extension.InterceptorPoint { return extension.PointCompactionLive }

func (p *LiveCompactionPayload) Validate() error {
	if p.Context != nil || p.Candidates != nil || p.Cut == nil || p.Cut.Start == nil || p.Cut.End == nil ||
		*p.Cut.Start < 0 || *p.Cut.End <= *p.Cut.Start {
		return errors.New("live compaction replacement must specify only one valid cut")
	}
	return nil
}

func (p *LiveCompactionPayload) adoptProposal(old *LiveCompactionPayload) error {
	if !slices.ContainsFunc(old.Candidates, func(c LiveCompactionCandidate) bool {
		return c.Start == *p.Cut.Start && c.End == *p.Cut.End
	}) {
		return errors.New("cut does not match an offered live compaction range")
	}
	p.Context, p.Candidates = old.Context, old.Candidates
	return nil
}

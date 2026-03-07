package requestingapp

import (
	"path/filepath"

	"xaa-mcp-demo/internal/shared/store"
	"xaa-mcp-demo/internal/shared/trace"
)

type State struct {
	Flows []trace.Flow `json:"flows"`
}

type Store struct {
	json *store.JSONStore[State]
}

func NewStore(dataDir string) *Store {
	return &Store{
		json: store.NewJSONStore(filepath.Join(dataDir, "requesting-app-state.json"), func(state *State) {
			state.Flows = []trace.Flow{}
		}),
	}
}

func (s *Store) SaveFlow(flow trace.Flow) error {
	_, err := s.json.Update(func(state *State) error {
		state.Flows = append([]trace.Flow{flow}, state.Flows...)
		if len(state.Flows) > 25 {
			state.Flows = state.Flows[:25]
		}
		return nil
	})
	return err
}

func (s *Store) ListFlows() ([]trace.Flow, error) {
	state, err := s.json.Read()
	if err != nil {
		return nil, err
	}
	return append([]trace.Flow(nil), state.Flows...), nil
}

func (s *Store) LatestFlow() (*trace.Flow, error) {
	flows, err := s.ListFlows()
	if err != nil {
		return nil, err
	}
	if len(flows) == 0 {
		return nil, nil
	}
	return &flows[0], nil
}

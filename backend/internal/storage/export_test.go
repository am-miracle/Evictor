package storage

import "github.com/jackc/pgx/v5/pgxpool"

// Raw pool access for integration tests that set up or assert on state no typed
// method should exist for. Compiled only into the test binary, so production
// callers cannot reach past Store's interface.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

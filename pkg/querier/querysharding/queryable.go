package querysharding

import (
	"context"
	"encoding/hex"
	"github.com/cortexproject/cortex/pkg/querier/astmapper"
	"github.com/cortexproject/cortex/pkg/querier/queryrange"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

// DownstreamQueryable is a wrapper for and implementor of the Queryable interface.
type DownstreamQueryable struct {
	Req     *queryrange.Request
	Handler queryrange.Handler
}

func (q *DownstreamQueryable) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return &DownstreamQuerier{ctx, q.Req, q.Handler}, nil
}

// DownstreamQueryable is a an implementor of the Queryable interface.
type DownstreamQuerier struct {
	Ctx     context.Context
	Req     *queryrange.Request
	Handler queryrange.Handler
}

// Select returns a set of series that matches the given label matchers.
func (q *DownstreamQuerier) Select(
	sp *storage.SelectParams,
	matchers ...*labels.Matcher,
) (storage.SeriesSet, storage.Warnings, error) {
	for _, matcher := range matchers {
		if matcher.Name == astmapper.EMBEDDED_QUERY_FLAG {
			// this is an embedded query
			return q.handleEmbeddedQuery(matcher.Value)
		}
	}

	return nil, nil, errors.Errorf("DownstreamQuerier cannot handle a non-embedded query")
}

// handleEmbeddedQuery defers execution of an encoded query to a downstream Handler
func (q *DownstreamQuerier) handleEmbeddedQuery(encoded string) (storage.SeriesSet, storage.Warnings, error) {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, nil, err
	}

	resp, err := q.Handler.Do(q.Ctx, ReplaceQuery(*q.Req, string(decoded)))
	if err != nil {
		return nil, nil, err
	}

	if resp.Error != "" {
		return nil, nil, errors.Errorf(resp.Error)
	}

	set, err := ResponseToSeries(resp.Data)
	return set, nil, err

}

// other storage.Querier impls that are not used by engine
// LabelValues returns all potential values for a label name.
func (q *DownstreamQuerier) LabelValues(name string) ([]string, storage.Warnings, error) {
	return nil, nil, errors.Errorf("unimplemented")
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (q *DownstreamQuerier) LabelNames() ([]string, storage.Warnings, error) {
	return nil, nil, errors.Errorf("unimplemented")
}

// Close releases the resources of the Querier.
func (q *DownstreamQuerier) Close() error {
	return nil
}

// take advantage of pass by value to clone a request with a new query
func ReplaceQuery(req queryrange.Request, query string) *queryrange.Request {
	req.Query = query
	return &req
}
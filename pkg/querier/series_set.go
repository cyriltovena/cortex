// Some of the code in this file was adapted from Prometheus (https://github.com/prometheus/prometheus).
// The original license header is included below:
//
// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package querier

import (
	"sort"

	"github.com/cortexproject/cortex/pkg/prom1/storage/metric"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

// ConcreteSeriesSet implements storage.SeriesSet.
type ConcreteSeriesSet struct {
	cur    int
	series []storage.Series
}

func NewConcreteSeriesSet(series []storage.Series) storage.SeriesSet {
	sort.Sort(byLabels(series))
	return &ConcreteSeriesSet{
		cur:    -1,
		series: series,
	}
}

func (c *ConcreteSeriesSet) Next() bool {
	c.cur++
	return c.cur < len(c.series)
}

func (c *ConcreteSeriesSet) At() storage.Series {
	return c.series[c.cur]
}

func (c *ConcreteSeriesSet) Err() error {
	return nil
}

// ConcreteSeries implements storage.Series.
type ConcreteSeries struct {
	labels  labels.Labels
	samples []model.SamplePair
}

func NewConcreteSeries(ls labels.Labels, samples []model.SamplePair) *ConcreteSeries {
	return &ConcreteSeries{
		labels:  ls,
		samples: samples,
	}
}

func (c *ConcreteSeries) Labels() labels.Labels {
	return c.labels
}

func (c *ConcreteSeries) Iterator() storage.SeriesIterator {
	return newConcreteSeriesIterator(c)
}

// concreteSeriesIterator implements storage.SeriesIterator.
type concreteSeriesIterator struct {
	cur    int
	series *ConcreteSeries
}

func newConcreteSeriesIterator(series *ConcreteSeries) storage.SeriesIterator {
	return &concreteSeriesIterator{
		cur:    -1,
		series: series,
	}
}

func (c *concreteSeriesIterator) Seek(t int64) bool {
	c.cur = sort.Search(len(c.series.samples), func(n int) bool {
		return c.series.samples[n].Timestamp >= model.Time(t)
	})
	return c.cur < len(c.series.samples)
}

func (c *concreteSeriesIterator) At() (t int64, v float64) {
	s := c.series.samples[c.cur]
	return int64(s.Timestamp), float64(s.Value)
}

func (c *concreteSeriesIterator) Next() bool {
	c.cur++
	return c.cur < len(c.series.samples)
}

func (c *concreteSeriesIterator) Err() error {
	return nil
}

// errIterator implements storage.SeriesIterator, just returning an error.
type errIterator struct {
	err error
}

func (errIterator) Seek(int64) bool {
	return false
}

func (errIterator) Next() bool {
	return false
}

func (errIterator) At() (t int64, v float64) {
	return 0, 0
}

func (e errIterator) Err() error {
	return e.err
}

func matrixToSeriesSet(m model.Matrix) storage.SeriesSet {
	series := make([]storage.Series, 0, len(m))
	for _, ss := range m {
		series = append(series, &ConcreteSeries{
			labels:  metricToLabels(ss.Metric),
			samples: ss.Values,
		})
	}
	return NewConcreteSeriesSet(series)
}

func metricsToSeriesSet(ms []metric.Metric) storage.SeriesSet {
	series := make([]storage.Series, 0, len(ms))
	for _, m := range ms {
		series = append(series, &ConcreteSeries{
			labels:  metricToLabels(m.Metric),
			samples: nil,
		})
	}
	return NewConcreteSeriesSet(series)
}

func metricToLabels(m model.Metric) labels.Labels {
	ls := make(labels.Labels, 0, len(m))
	for k, v := range m {
		ls = append(ls, labels.Label{
			Name:  string(k),
			Value: string(v),
		})
	}
	// PromQL expects all labels to be sorted! In general, anyone constructing
	// a labels.Labels list is responsible for sorting it during construction time.
	sort.Sort(ls)
	return ls
}

type byLabels []storage.Series

func (b byLabels) Len() int           { return len(b) }
func (b byLabels) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byLabels) Less(i, j int) bool { return labels.Compare(b[i].Labels(), b[j].Labels()) < 0 }

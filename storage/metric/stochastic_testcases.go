// Copyright 2013 Prometheus Team
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

package metric

import (
	"fmt"
	"github.com/prometheus/prometheus/model"
	"github.com/prometheus/prometheus/utility/test"
	"math"
	"math/rand"
	"testing/quick"
	"time"
)

const (
	stochasticMaximumVariance = 8
)

func BasicLifecycleTests(p MetricPersistence, t test.Tester) {
	if p == nil {
		t.Errorf("Received nil Metric Persistence.\n")
		return
	}
}

func ReadEmptyTests(p MetricPersistence, t test.Tester) {
	hasLabelPair := func(x int) (success bool) {
		name := model.LabelName(string(x))
		value := model.LabelValue(string(x))

		labelSet := model.LabelSet{
			name: value,
		}

		fingerprints, err := p.GetFingerprintsForLabelSet(labelSet)
		if err != nil {
			t.Error(err)
			return
		}

		success = len(fingerprints) == 0
		if !success {
			t.Errorf("unexpected fingerprint length %d, got %d", 0, len(fingerprints))
		}

		return
	}

	err := quick.Check(hasLabelPair, nil)
	if err != nil {
		t.Error(err)
		return
	}

	hasLabelName := func(x int) (success bool) {
		labelName := model.LabelName(string(x))

		fingerprints, err := p.GetFingerprintsForLabelName(labelName)
		if err != nil {
			t.Error(err)
			return
		}

		success = len(fingerprints) == 0
		if !success {
			t.Errorf("unexpected fingerprint length %d, got %d", 0, len(fingerprints))
		}

		return
	}

	err = quick.Check(hasLabelName, nil)
	if err != nil {
		t.Error(err)
		return
	}
}

func AppendSampleAsPureSparseAppendTests(p MetricPersistence, t test.Tester) {
	appendSample := func(x int) (success bool) {
		v := model.SampleValue(x)
		ts := time.Unix(int64(x), int64(x))
		labelName := model.LabelName(x)
		labelValue := model.LabelValue(x)
		l := model.Metric{labelName: labelValue}

		sample := model.Sample{
			Value:     v,
			Timestamp: ts,
			Metric:    l,
		}

		err := p.AppendSample(sample)

		success = err == nil
		if !success {
			t.Error(err)
		}

		return
	}

	if err := quick.Check(appendSample, nil); err != nil {
		t.Error(err)
	}
}

func AppendSampleAsSparseAppendWithReadsTests(p MetricPersistence, t test.Tester) {
	appendSample := func(x int) (success bool) {
		v := model.SampleValue(x)
		ts := time.Unix(int64(x), int64(x))
		labelName := model.LabelName(x)
		labelValue := model.LabelValue(x)
		l := model.Metric{labelName: labelValue}

		sample := model.Sample{
			Value:     v,
			Timestamp: ts,
			Metric:    l,
		}

		err := p.AppendSample(sample)
		if err != nil {
			t.Error(err)
			return
		}

		fingerprints, err := p.GetFingerprintsForLabelName(labelName)
		if err != nil {
			t.Error(err)
			return
		}
		if len(fingerprints) != 1 {
			t.Errorf("expected fingerprint count of %d, got %d", 1, len(fingerprints))
			return
		}

		fingerprints, err = p.GetFingerprintsForLabelSet(model.LabelSet{
			labelName: labelValue,
		})
		if err != nil {
			t.Error(err)
			return
		}
		if len(fingerprints) != 1 {
			t.Error("expected fingerprint count of %d, got %d", 1, len(fingerprints))
			return
		}

		return true
	}

	if err := quick.Check(appendSample, nil); err != nil {
		t.Error(err)
	}
}

func AppendSampleAsPureSingleEntityAppendTests(p MetricPersistence, t test.Tester) {
	appendSample := func(x int) bool {
		sample := model.Sample{
			Value:     model.SampleValue(x),
			Timestamp: time.Unix(int64(x), 0),
			Metric:    model.Metric{"name": "my_metric"},
		}

		err := p.AppendSample(sample)

		return err == nil
	}

	if err := quick.Check(appendSample, nil); err != nil {
		t.Error(err)
	}
}

func StochasticTests(persistenceMaker func() MetricPersistence, t test.Tester) {
	stochastic := func(x int) (success bool) {
		p := persistenceMaker()
		defer func() {
			err := p.Close()
			if err != nil {
				t.Error(err)
			}
		}()

		seed := rand.NewSource(int64(x))
		random := rand.New(seed)

		numberOfMetrics := random.Intn(stochasticMaximumVariance) + 1
		numberOfSharedLabels := random.Intn(stochasticMaximumVariance)
		numberOfUnsharedLabels := random.Intn(stochasticMaximumVariance)
		numberOfSamples := random.Intn(stochasticMaximumVariance) + 2
		numberOfRangeScans := random.Intn(stochasticMaximumVariance)

		metricTimestamps := map[int]map[int64]bool{}
		metricEarliestSample := map[int]int64{}
		metricNewestSample := map[int]int64{}

		for metricIndex := 0; metricIndex < numberOfMetrics; metricIndex++ {
			sample := model.Sample{
				Metric: model.Metric{},
			}

			v := model.LabelValue(fmt.Sprintf("metric_index_%d", metricIndex))
			sample.Metric["name"] = v

			for sharedLabelIndex := 0; sharedLabelIndex < numberOfSharedLabels; sharedLabelIndex++ {
				l := model.LabelName(fmt.Sprintf("shared_label_%d", sharedLabelIndex))
				v := model.LabelValue(fmt.Sprintf("label_%d", sharedLabelIndex))

				sample.Metric[l] = v
			}

			for unsharedLabelIndex := 0; unsharedLabelIndex < numberOfUnsharedLabels; unsharedLabelIndex++ {
				l := model.LabelName(fmt.Sprintf("metric_index_%d_private_label_%d", metricIndex, unsharedLabelIndex))
				v := model.LabelValue(fmt.Sprintf("private_label_%d", unsharedLabelIndex))

				sample.Metric[l] = v
			}

			timestamps := map[int64]bool{}
			metricTimestamps[metricIndex] = timestamps
			var (
				newestSample  int64 = math.MinInt64
				oldestSample  int64 = math.MaxInt64
				nextTimestamp func() int64
			)

			nextTimestamp = func() int64 {
				var candidate int64
				candidate = random.Int63n(math.MaxInt32 - 1)

				if _, has := timestamps[candidate]; has {
					// WART
					candidate = nextTimestamp()
				}

				timestamps[candidate] = true

				if candidate < oldestSample {
					oldestSample = candidate
				}

				if candidate > newestSample {
					newestSample = candidate
				}

				return candidate
			}

			for sampleIndex := 0; sampleIndex < numberOfSamples; sampleIndex++ {
				sample.Timestamp = time.Unix(nextTimestamp(), 0)
				sample.Value = model.SampleValue(sampleIndex)

				err := p.AppendSample(sample)

				if err != nil {
					t.Error(err)
					return
				}
			}

			metricEarliestSample[metricIndex] = oldestSample
			metricNewestSample[metricIndex] = newestSample

			for sharedLabelIndex := 0; sharedLabelIndex < numberOfSharedLabels; sharedLabelIndex++ {
				labelPair := model.LabelSet{
					model.LabelName(fmt.Sprintf("shared_label_%d", sharedLabelIndex)): model.LabelValue(fmt.Sprintf("label_%d", sharedLabelIndex)),
				}

				fingerprints, err := p.GetFingerprintsForLabelSet(labelPair)
				if err != nil {
					t.Error(err)
					return
				}
				if len(fingerprints) == 0 {
					t.Errorf("expected fingerprint count of %d, got %d", 0, len(fingerprints))
					return
				}

				labelName := model.LabelName(fmt.Sprintf("shared_label_%d", sharedLabelIndex))
				fingerprints, err = p.GetFingerprintsForLabelName(labelName)
				if err != nil {
					t.Error(err)
					return
				}
				if len(fingerprints) == 0 {
					t.Errorf("expected fingerprint count of %d, got %d", 0, len(fingerprints))
					return
				}
			}
		}

		for sharedIndex := 0; sharedIndex < numberOfSharedLabels; sharedIndex++ {
			labelName := model.LabelName(fmt.Sprintf("shared_label_%d", sharedIndex))
			fingerprints, err := p.GetFingerprintsForLabelName(labelName)
			if err != nil {
				t.Error(err)
				return
			}

			if len(fingerprints) != numberOfMetrics {
				t.Errorf("expected fingerprint count of %d, got %d", numberOfMetrics, len(fingerprints))
				return
			}
		}

		for metricIndex := 0; metricIndex < numberOfMetrics; metricIndex++ {
			for unsharedLabelIndex := 0; unsharedLabelIndex < numberOfUnsharedLabels; unsharedLabelIndex++ {
				labelName := model.LabelName(fmt.Sprintf("metric_index_%d_private_label_%d", metricIndex, unsharedLabelIndex))
				labelValue := model.LabelValue(fmt.Sprintf("private_label_%d", unsharedLabelIndex))
				labelSet := model.LabelSet{
					labelName: labelValue,
				}

				fingerprints, err := p.GetFingerprintsForLabelSet(labelSet)
				if err != nil {
					t.Error(err)
					return
				}
				if len(fingerprints) != 1 {
					t.Errorf("expected fingerprint count of %d, got %d", 1, len(fingerprints))
					return
				}

				fingerprints, err = p.GetFingerprintsForLabelName(labelName)
				if err != nil {
					t.Error(err)
					return
				}
				if len(fingerprints) != 1 {
					t.Errorf("expected fingerprint count of %d, got %d", 1, len(fingerprints))
					return
				}
			}

			metric := model.Metric{}
			metric["name"] = model.LabelValue(fmt.Sprintf("metric_index_%d", metricIndex))

			for i := 0; i < numberOfSharedLabels; i++ {
				l := model.LabelName(fmt.Sprintf("shared_label_%d", i))
				v := model.LabelValue(fmt.Sprintf("label_%d", i))

				metric[l] = v
			}

			for i := 0; i < numberOfUnsharedLabels; i++ {
				l := model.LabelName(fmt.Sprintf("metric_index_%d_private_label_%d", metricIndex, i))
				v := model.LabelValue(fmt.Sprintf("private_label_%d", i))

				metric[l] = v
			}

			for i := 0; i < numberOfRangeScans; i++ {
				timestamps := metricTimestamps[metricIndex]

				var first int64 = 0
				var second int64 = 0

				for {
					firstCandidate := random.Int63n(int64(len(timestamps)))
					secondCandidate := random.Int63n(int64(len(timestamps)))

					smallest := int64(-1)
					largest := int64(-1)

					if firstCandidate == secondCandidate {
						continue
					} else if firstCandidate > secondCandidate {
						largest = firstCandidate
						smallest = secondCandidate
					} else {
						largest = secondCandidate
						smallest = firstCandidate
					}

					j := int64(0)
					for i := range timestamps {
						if j == smallest {
							first = i
						} else if j == largest {
							second = i
							break
						}
						j++
					}

					break
				}

				begin := first
				end := second

				if second < first {
					begin, end = second, first
				}

				interval := model.Interval{
					OldestInclusive: time.Unix(begin, 0),
					NewestInclusive: time.Unix(end, 0),
				}

				samples, err := p.GetRangeValues(metric, interval)
				if err != nil {
					t.Error(err)
					return
				}

				if len(samples.Values) < 2 {
					t.Errorf("expected sample count less than %d, got %d", 2, len(samples.Values))
					return
				}
			}
		}

		return true
	}

	if err := quick.Check(stochastic, nil); err != nil {
		t.Error(err)
	}
}

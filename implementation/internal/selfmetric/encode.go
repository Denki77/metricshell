package selfmetric

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type TextFormat string

const (
	PrometheusText  TextFormat = "prometheus"
	OpenMetricsText TextFormat = "openmetrics"
)

func (registry *Registry) Encode(format TextFormat) ([]byte, error) {
	return Encode(registry.View(), format)
}

// Encode renders one immutable self-metric view without application filtering.
func Encode(view View, format TextFormat) ([]byte, error) {
	if format != PrometheusText && format != OpenMetricsText {
		return nil, ErrValue
	}
	var output bytes.Buffer
	for _, family := range view.Families {
		if family.Type != Gauge && family.Type != Counter && family.Type != Histogram {
			return nil, ErrMetricType
		}
		metadataName := family.Name
		if format == OpenMetricsText && family.Type == Counter {
			metadataName = strings.TrimSuffix(metadataName, "_total")
		}
		fmt.Fprintf(&output, "# HELP %s %s\n", metadataName, escapeHelp(family.Help))
		fmt.Fprintf(&output, "# TYPE %s %s\n", metadataName, family.Type)
		for _, sample := range family.Samples {
			switch family.Type {
			case Gauge:
				writeSample(&output, family.Name, sample.Labels, formatFloat(sample.Gauge))
			case Counter:
				writeSample(&output, family.Name, sample.Labels, strconv.FormatUint(sample.Counter, 10))
			case Histogram:
				if sample.Histogram == nil {
					return nil, ErrMetricType
				}
				for _, bucket := range sample.Histogram.Buckets {
					labels := append(cloneLabels(sample.Labels), Label{Name: "le", Value: formatFloat(bucket.UpperBound)})
					writeSample(&output, family.Name+"_bucket", labels, strconv.FormatUint(bucket.Count, 10))
				}
				labels := append(cloneLabels(sample.Labels), Label{Name: "le", Value: "+Inf"})
				writeSample(&output, family.Name+"_bucket", labels, strconv.FormatUint(sample.Histogram.Count, 10))
				writeSample(&output, family.Name+"_sum", sample.Labels, formatFloat(sample.Histogram.Sum))
				writeSample(&output, family.Name+"_count", sample.Labels, strconv.FormatUint(sample.Histogram.Count, 10))
			}
		}
	}
	if format == OpenMetricsText {
		output.WriteString("# EOF\n")
	}
	return output.Bytes(), nil
}

func writeSample(output *bytes.Buffer, name string, labels []Label, value string) {
	output.WriteString(name)
	if len(labels) > 0 {
		output.WriteByte('{')
		for index, label := range labels {
			if index > 0 {
				output.WriteByte(',')
			}
			fmt.Fprintf(output, `%s="%s"`, label.Name, escapeLabelValue(label.Value))
		}
		output.WriteByte('}')
	}
	output.WriteByte(' ')
	output.WriteString(value)
	output.WriteByte('\n')
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

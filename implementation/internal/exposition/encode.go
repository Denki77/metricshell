package exposition

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func Encode(application snapshot.ValidatedSnapshot, metrics selfmetric.View, filter FamilyFilter, format Format) ([]byte, error) {
	metricFormat, err := selfMetricFormat(format)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	for _, family := range application.Families() {
		if filter != nil && !filter(family) {
			continue
		}
		writeApplicationFamily(&output, family)
	}
	self, err := selfmetric.Encode(metrics, metricFormat)
	if err != nil {
		return nil, err
	}
	if format == OpenMetrics {
		self = bytes.TrimSuffix(self, []byte("# EOF\n"))
	}
	output.Write(self)
	if format == OpenMetrics {
		output.WriteString("# EOF\n")
	}
	return output.Bytes(), nil
}

func selfMetricFormat(format Format) (selfmetric.TextFormat, error) {
	switch format {
	case Prometheus:
		return selfmetric.PrometheusText, nil
	case OpenMetrics:
		return selfmetric.OpenMetricsText, nil
	default:
		return "", EncodingErrorValue
	}
}

func writeApplicationFamily(output *bytes.Buffer, family snapshot.Family) {
	output.WriteString("# HELP ")
	output.WriteString(family.Name())
	output.WriteByte(' ')
	output.WriteString(escapeHelp(family.Help()))
	output.WriteByte('\n')
	output.WriteString("# TYPE ")
	output.WriteString(family.Name())
	output.WriteByte(' ')
	output.WriteString(string(family.Type()))
	output.WriteByte('\n')
	for _, series := range family.Series() {
		switch family.Type() {
		case snapshot.Counter:
			writeApplicationSample(output, family.Name()+"_total", series.Labels(), "", "", series.Value())
		case snapshot.Gauge:
			writeApplicationSample(output, family.Name(), series.Labels(), "", "", series.Value())
		case snapshot.Histogram:
			histogram, ok := series.Histogram()
			if !ok {
				continue
			}
			for _, bucket := range histogram.Buckets() {
				writeApplicationSample(output, family.Name()+"_bucket", series.Labels(), "le", bucket.UpperBound(), strconv.FormatUint(bucket.Count(), 10))
			}
			writeApplicationSample(output, family.Name()+"_bucket", series.Labels(), "le", "+Inf", strconv.FormatUint(histogram.Count(), 10))
			writeApplicationSample(output, family.Name()+"_sum", series.Labels(), "", "", histogram.Sum())
			writeApplicationSample(output, family.Name()+"_count", series.Labels(), "", "", strconv.FormatUint(histogram.Count(), 10))
		}
	}
}

func writeApplicationSample(output *bytes.Buffer, name string, labels []snapshot.Label, extraName, extraValue, value string) {
	output.WriteString(name)
	if len(labels) > 0 || extraName != "" {
		output.WriteByte('{')
		for index, label := range labels {
			if index > 0 {
				output.WriteByte(',')
			}
			output.WriteString(label.Name())
			output.WriteString("=\"")
			output.WriteString(escapeLabel(label.Value()))
			output.WriteByte('"')
		}
		if extraName != "" {
			if len(labels) > 0 {
				output.WriteByte(',')
			}
			output.WriteString(extraName)
			output.WriteString("=\"")
			output.WriteString(escapeLabel(extraValue))
			output.WriteByte('"')
		}
		output.WriteByte('}')
	}
	output.WriteByte(' ')
	output.WriteString(value)
	output.WriteByte('\n')
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

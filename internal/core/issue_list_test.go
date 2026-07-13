package core

import (
	"reflect"
	"testing"
)

func TestNormalizeIssueListValues(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    []string
		wantErr bool
	}{
		{name: "missing", values: nil, want: nil},
		{name: "csv and repeated values", values: []string{" afc, aion ", "afc"}, want: []string{"afc", "aion"}},
		{name: "leading comma", values: []string{",afc"}, wantErr: true},
		{name: "trailing comma", values: []string{"afc,"}, wantErr: true},
		{name: "doubled comma", values: []string{"afc,,aion"}, wantErr: true},
		{name: "only whitespace", values: []string{"  "}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeIssueListValues(tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeIssueListValues(%q) error = %v, wantErr %v", tt.values, err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeIssueListValues(%q) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

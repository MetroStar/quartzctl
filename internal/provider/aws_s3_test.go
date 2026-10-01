package provider

import "testing"

func TestS3CreateRegion(t *testing.T) {
	tests := []struct {
		name   string
		region string
		want   string
	}{
		{name: "commercial default", region: "us-east-1", want: "us-east-1"},
		{name: "commercial regional bucket", region: "us-west-2", want: "us-east-1"},
		{name: "govcloud west", region: "us-gov-west-1", want: "us-gov-west-1"},
		{name: "govcloud east", region: "us-gov-east-1", want: "us-gov-east-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := s3CreateRegion(test.region); got != test.want {
				t.Fatalf("s3CreateRegion(%q) = %q, want %q", test.region, got, test.want)
			}
		})
	}
}

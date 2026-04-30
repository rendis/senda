package domain

import "testing"

func TestSubscriptionSource_Valid(t *testing.T) {
	cases := []struct {
		in   SubscriptionSource
		want bool
	}{
		{SubscriptionSourceRecipientOptout, true},
		{SubscriptionSourceRecipientOptin, true},
		{SubscriptionSourceAdmin, true},
		{"", false},
		{"unknown", false},
		{"RECIPIENT_OPTOUT", false}, // case-sensitive
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.want {
			t.Errorf("Valid(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

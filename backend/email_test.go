package backend

import "testing"

func TestSendEmail(t *testing.T) {
	tests := []struct {
		name    string
		emailer Emailer
		to      string
		subject string
		body    string
	}{
		{
			name: "test 1",
			emailer: Emailer{
				Server: "localhost:25",
				From:   "notify@q2admin.org",
			},
			to:      "joe@joereid.com",
			subject: "Test email",
			body:    "This is just a test email",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.emailer.Send(tc.to, tc.subject, tc.body)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

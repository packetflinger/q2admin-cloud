package backend

import (
	"fmt"
	"net/smtp"
)

type Emailer struct {
	Server string // addr:port
	From   string
}

func (e *Emailer) Send(recipient, subject, body string) error {
	c, err := smtp.Dial(e.Server)
	if err != nil {
		return fmt.Errorf("error dialing smtp server: %v", err)
	}
	if err := c.Mail(e.From); err != nil {
		return fmt.Errorf("error setting email sender: %v", err)
	}
	if err := c.Rcpt(recipient); err != nil {
		return fmt.Errorf("error setting email recipient: %v", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("error sending DATA command: %v", err)
	}
	_, err = fmt.Fprintln(wc, body)
	if err != nil {
		return fmt.Errorf("error writing email body: %v", err)
	}
	err = wc.Close()
	if err != nil {
		return fmt.Errorf("error closing email body writer: %v", err)
	}
	err = c.Quit()
	if err != nil {
		return fmt.Errorf("error sending email QUIT command: %v", err)
	}
	return nil
}

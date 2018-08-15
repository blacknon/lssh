package ssh

import (
	"fmt"
	"os"
)

func (r *Run) term() (err error) {
	server := r.ServerList[0]

	c := new(Connect)
	c.Server = server
	c.Conf = r.ConfList

	session, err := c.CreateSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", c.Server, err)
		return err
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	c.ConTerm(session)
	return
}

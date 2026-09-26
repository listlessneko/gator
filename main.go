package main

import (
	"fmt"
	"os"

	"codeberg.org/kedarui/gator/internal/config"
)

type state struct {
	config *config.Config
}

type command struct {
	name string
	args []string
}

type callbackMap map[string]func(s *state, cmd command) error

type commands struct {
	callback callbackMap
}

func (c *commands) run(s *state, cmd command) error {
	if s == nil {
		return fmt.Errorf("state does not exist")
	}
	err := c.callback[cmd.name](s, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (c *commands) register(name string, f func(s *state, cmd command) error) {
	c.callback[name] = f
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("no username provided")
	}
	username := cmd.args[0]
	err := s.config.SetUser(username)
	if err != nil {
		return err
	}
	fmt.Printf("username: '%s' has been set\n", username)
	return nil
}

func main() {
	content, err := config.Read()
	if err != nil {
		fmt.Println(err)
		return
	}

	s := &state{
		config: &content,
	}

	cmds := commands{
		callback: make(callbackMap),
	}

	cmds.register("login", handlerLogin)

	if len(os.Args) < 2 {
		fmt.Println("no arguments provided")
		os.Exit(1)
	}

	args := os.Args[1:]
	cmd := command{
		name: args[0],
		args: args[1:],
	}

	err = cmds.run(s, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

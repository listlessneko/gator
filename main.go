package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"codeberg.org/kedarui/gator/internal/config"
	"codeberg.org/kedarui/gator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db     *database.Queries
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

	fn, ok := c.callback[cmd.name]
	if !ok {
		return fmt.Errorf("command does not exist")
	}
	err := fn(s, cmd)
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

	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("no username found")
	}
	err = s.config.SetUser(username)
	if err != nil {
		return err
	}
	fmt.Printf("username: '%s' has been set\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("no username provided")
	}

	name := cmd.args[0]
	userParams := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
	}

	_, err := s.db.CreateUser(context.Background(), userParams)
	if err != nil {
		os.Exit(1)
	}
	s.config.SetUser(name)
	fmt.Printf("user '%s' was successfully created\n", name)
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

	db, err := sql.Open("postgres", s.config.DBURL)
	dbQueries := database.New(db)
	s.db = dbQueries

	cmds := commands{
		callback: make(callbackMap),
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)

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

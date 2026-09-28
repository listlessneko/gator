package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
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

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}
	var names []string
	for _, u := range users {
		name := "* " + u.Name
		if u.Name == s.config.CurrentUserName {
			name += " (current)"
		}
		names = append(names, name)
	}
	final := strings.Join(names, "\n")
	fmt.Println(final)
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUser(context.Background())
	if err != nil {
		return err
	}
	fmt.Println("successfully resetted users table")
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req.Header.Set("User-Agent", "gator")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rssFeed *RSSFeed
	err = xml.Unmarshal(data, &rssFeed)
	if err != nil {
		return nil, err
	}

	return rssFeed, nil
}

func handlerAgg(s *state, cmd command) error {
	url := "https://www.wagslane.dev/index.xml"
	feed, err := fetchFeed(context.Background(), url)
	if err != nil {
	return err
	}
	fmt.Printf("Title: %s\n", html.UnescapeString(feed.Channel.Title))
	fmt.Printf("Description: %s\n", html.UnescapeString(feed.Channel.Description))
	for _, item := range feed.Channel.Item {
		fmt.Printf("Title: %s\n", html.UnescapeString(item.Title))
		fmt.Printf("Description: %s\n", html.UnescapeString(item.Description))
	}
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
	cmds.register("users", handlerUsers)
	cmds.register("reset", handlerReset)
	cmds.register("agg", handlerAgg)

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

package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"codeberg.org/kedarui/gator/internal/config"
	"codeberg.org/kedarui/gator/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
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
	if len(cmd.args) < 1 {
		return fmt.Errorf("no time provided")
	}
	timeBetweenRequests := cmd.args[0]

	timeParsed, err := time.ParseDuration(timeBetweenRequests)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(timeParsed)
	for ; ; <-ticker.C {
		fmt.Printf("Collecting feeds every %s\n", timeBetweenRequests)
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("missing argument(s)")
	}
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return err
	}

	feedParams := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	}

	feed, err := s.db.CreateFeed(context.Background(), feedParams)
	if err != nil {
		return err
	}

	feedFollowParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return err
	}

	fmt.Printf("Feed: %+v\n", feed)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}
	for _, f := range feeds {
		fmt.Printf("Feed Name: %s\n", f.Name)
		fmt.Printf("Feed URL: %s\n", f.Url)
		user, err := s.db.GetUserById(context.Background(), f.UserID)
		if err != nil {
			fmt.Println("Created By: Unknown")
			return err
		}
		fmt.Printf("Created By: %s\n", user.Name)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("no feed provided")
	}

	url := cmd.args[0]

	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return err
	}

	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return err
	}

	feedFollowParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	feedFollow, err := s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return err
	}
	fmt.Println("successfully followed feed")
	fmt.Printf("Feed Name: %s\n", feedFollow.FeedName)
	fmt.Printf("Followed By: %s\n", feedFollow.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
	if err != nil {
		return err
	}

	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	fmt.Println("Following:")
	for _, ff := range feedFollows {
		fmt.Printf("%s\n", ff.FeedName)
	}
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("no url provided")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeedByUrl(context.Background(), url)
	if err != nil {
		return err
	}

	deleteFeedParams := database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}

	err = s.db.DeleteFeedFollow(context.Background(), deleteFeedParams)
	if err != nil {
		return err
	}

	return nil
}

func createPost(s *state, feed database.Feed, item RSSItem) error {
	published_at, _ := time.Parse(time.RFC1123Z, item.PubDate)
	createPostParams := database.CreatePostParams{
		ID:          uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       item.Title,
		Url:         item.Link,
		Description: item.Description,
		PublishedAt: sql.NullTime{
			Time:  published_at,
			Valid: true,
		},
		FeedID: feed.ID,
	}
	_, err := s.db.CreatePost(context.Background(), createPostParams)
	pqErr := pq.As(err, pqerror.UniqueViolation)
	if pqErr != nil {
		fmt.Printf("link %s already exists\n", item.Link)
	}
	return nil
}

func scrapeFeeds(s *state) error {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	fetchedFeed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		return err
	}

	markFeedFetchedParams := database.MarkFeedFetchedParams{
		UpdatedAt: time.Now(),
		LastFetchedAt: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
		ID: nextFeed.ID,
	}

	_, err = s.db.MarkFeedFetched(context.Background(), markFeedFetchedParams)
	if err != nil {
		return err
	}

	for _, i := range fetchedFeed.Channel.Item {
		createPost(s, nextFeed, i)
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
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))

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
